# -*- coding: utf-8 -*-

"""
角色卡本地化工具
localizer.py - 资源查找、下载和替换的核心逻辑 (v3.0 - 遵照原始逻辑修复版)
"""

import re
import requests
import threading
import hashlib
from pathlib import Path
from urllib.parse import urlparse, quote
from concurrent.futures import ThreadPoolExecutor, as_completed
from requests.adapters import HTTPAdapter
from urllib3.util.retry import Retry

import importlib.util

try:
    # 检查 socks 模块是否可用
    if importlib.util.find_spec("socks") and importlib.util.find_spec("socket"):
        import socks

        # 进一步检查是否是 PySocks，因为它提供了 SOCKS5 常量
        if hasattr(socks, "SOCKS5"):
            SOCKS_SUPPORTED = True
        else:
            SOCKS_SUPPORTED = False
    else:
        SOCKS_SUPPORTED = False
except ImportError:
    SOCKS_SUPPORTED = False

MAX_WORKERS = 8

# 定义可接受的资源文件扩展名白名单
RESOURCE_EXTENSIONS = {
    # Images
    ".png",
    ".jpg",
    ".jpeg",
    ".gif",
    ".webp",
    ".bmp",
    ".svg",
    # Audio
    ".mp3",
    ".wav",
    ".ogg",
    ".m4a",
    ".flac",
    ".mid",
    # Video
    ".mp4",
    ".webm",
    ".mov",
    ".avi",
    # Fonts
    ".woff",
    ".woff2",
    ".ttf",
    ".otf",
    # Styles & Scripts
    ".css",
    ".js",
    # Data
    ".json",
    ".txt",
}


def create_session(proxies=None):
    session = requests.Session()
    retries = Retry(total=2, backoff_factor=0.5, status_forcelist=[500, 502, 503, 504])
    adapter = HTTPAdapter(max_retries=retries)
    session.mount("http://", adapter)
    session.mount("https", adapter)
    if proxies:
        session.proxies = proxies
    session.headers.update({"User-Agent": "Mozilla/5.0"})
    return session


def download_resource(task_info):
    url, local_path, proxies, force_proxy = task_info.values()

    # 简单地对URL中的空格等进行编码，大多数现代库能处理好其他字符
    encoded_url = quote(url, safe="/:@?=&")

    local_path.parent.mkdir(parents=True, exist_ok=True)
    if local_path.exists():
        try:
            content = local_path.read_bytes()
            return {
                "status": "skipped",
                "method": "文件已存在",
                "url": url,
                "local_path": local_path,
                "content": content,
            }
        except Exception:
            # 如果读取已存在文件失败，当作不存在处理
            pass

    session_args = {"timeout": 15}
    if not force_proxy:
        try:
            response = create_session().get(encoded_url, **session_args)
            response.raise_for_status()
            local_path.write_bytes(response.content)
            return {
                "status": "success",
                "method": "直连",
                "url": url,
                "local_path": local_path,
                "content": response.content,
            }
        except requests.RequestException:
            pass

    if proxies:
        try:
            method = "强制代理" if force_proxy else "代理回退"
            session_args["timeout"] = 45
            response = create_session(proxies=proxies).get(encoded_url, **session_args)
            response.raise_for_status()
            local_path.write_bytes(response.content)
            return {
                "status": "success",
                "method": method,
                "url": url,
                "local_path": local_path,
                "content": response.content,
            }
        except requests.RequestException as e:
            error_message = str(e).splitlines()[0] if str(e) else "未知错误"
            return {
                "status": "failure",
                "url": url,
                "error": f"代理失败: {error_message}",
            }

    return {"status": "failure", "url": url, "error": "直连失败且未提供代理"}


class Localizer:
    def __init__(self, card_data, output_dir, proxy=None, force_proxy_domains=None):
        self.card_data = card_data
        self.output_dir = Path(output_dir)
        # 从 output_dir 推断角色名，用于构建 web 路径
        self.safe_char_name = self.output_dir.name
        if (
            proxy
            and proxy.startswith(("socks5://", "socks5h://"))
            and not SOCKS_SUPPORTED
        ):
            raise ImportError(
                "检测到SOCKS代理，但未安装 PySocks 库。请运行 'pip install PySocks'。"
            )
        self.proxies = {"http": proxy, "https": proxy} if proxy else None
        self.force_proxy_domains = (
            force_proxy_domains if force_proxy_domains is not None else []
        )

        # 修正：在通用模式中明确排除换行符，防止跨行匹配
        self.url_pattern = re.compile(r'https?://[^\s\'"`<>(),\n\r]+')
        self.css_url_pattern = re.compile(r'url\((?:[\'"]?)(https?://.*?)(?:[\'"]?)\)')
        # 修正：在JS模式中也明确排除换行符
        self.js_url_pattern = re.compile(r'[\'"`](https?://[^\'"`\s\n\r]+)[\'"`]')
        # 用于从HTML中提取style标签内容的模式
        self.style_tag_pattern = re.compile(r"<style[^>]*>([\s\S]*?)</style>")

        self.successful_url_map = {}  # { "http://...": "/niko/char_name/file.css" }
        self.text_content_queue = []  # 待处理的文本内容
        self.processed_urls = set()  # 已处理过的URL，防止重复下载
        self._stop_event = threading.Event()  # 用于停止本地化进程的事件

    def get_resource_paths(self, url, context):
        """
        遵照参考脚本逻辑，为URL生成物理存储路径和Web访问路径。
        """
        try:
            parsed_url = urlparse(url)
            parsed_path = Path(parsed_url.path)

            # HTML/CSS 上下文逻辑 (来自 html资源本地化.py)
            if context in ["html", "css"]:
                url_hash = hashlib.sha1(url.encode()).hexdigest()[:12]
                file_ext = parsed_path.suffix or ".dat"
                if "googleapis.com/css" in url:
                    file_ext = ".css"

                filename = f"{url_hash}{file_ext}"
                # HTML资源直接放在角色目录下
                local_path = self.output_dir / filename
                # Web路径是绝对路径
                web_path = f"/niko/{self.safe_char_name}/{filename}"
                return local_path, web_path

            # JS 或其他上下文逻辑 (来自 js代码内置资源本地化.py)
            else:
                filename = parsed_path.name
                if not filename:  # 备用方案
                    filename = hashlib.sha1(url.encode()).hexdigest()[:12] + (
                        parsed_path.suffix or ".dat"
                    )

                # 根据文件类型确定子目录
                file_ext = parsed_path.suffix.lower()
                if file_ext in [
                    ".png",
                    ".jpg",
                    ".jpeg",
                    ".webp",
                    ".gif",
                    ".svg",
                    ".bmp",
                ]:
                    sub_dir_name = "images"
                elif file_ext in [
                    ".mp3",
                    ".wav",
                    ".ogg",
                    ".m4a",
                    ".flac",
                    ".mid",
                    ".mp4",
                    ".webm",
                    ".mov",
                    ".avi",
                ]:
                    sub_dir_name = "media"
                else:  # 字体, css, js, json等
                    sub_dir_name = "assets"

                local_path = self.output_dir / sub_dir_name / filename
                web_path = f"/niko/{self.safe_char_name}/{sub_dir_name}/{filename}"
                return local_path, web_path

        except Exception:
            return None, None

    def find_and_queue_urls(self, text_content, context):
        """根据上下文查找URL并加入下载队列"""
        raw_urls = set()
        # 1. 根据不同上下文使用不同策略
        if context == "css":
            raw_urls.update(
                m.group(1) for m in self.css_url_pattern.finditer(text_content)
            )
            raw_urls.update(
                self.url_pattern.findall(text_content)
            )  # CSS中也可能直接写URL
        elif context == "js":
            raw_urls.update(
                m.group(1) for m in self.js_url_pattern.finditer(text_content)
            )
        elif context == "html":
            # 提取HTML中的style标签内容，并加入处理队列
            for style_content in self.style_tag_pattern.findall(text_content):
                self.text_content_queue.append(
                    {"content": style_content, "context": "css"}
                )
            raw_urls.update(self.url_pattern.findall(text_content))
        else:  # json 或其他
            # 修正：处理JSON字符串化后，一个字段内含多个URL（以\n或\\n分隔）的情况
            potential_url_blobs = self.url_pattern.findall(text_content)
            for blob in potential_url_blobs:
                # 对每个文本块按换行符（包括转义的）进行分割
                urls_in_blob = re.split(r"[\n\r]+|\\n", blob)
                for url in urls_in_blob:
                    if url.strip():
                        raw_urls.add(url.strip())

        # 3. 清理URL (由于正则已修正，不再需要复杂的分割逻辑)
        tasks = []
        for url in raw_urls:
            cleaned_url = url.strip().rstrip("\\")
            if not cleaned_url or cleaned_url in self.processed_urls:
                continue

            # 4. 严格的URL过滤
            try:
                parsed_url = urlparse(url)
                # 必须是 http/https 协议，并且有域名
                if parsed_url.scheme not in ["http", "https"] or not parsed_url.netloc:
                    continue
                # 必须有路径，并且路径的扩展名在白名单内
                file_ext = Path(parsed_url.path).suffix.lower()
                if not file_ext or file_ext not in RESOURCE_EXTENSIONS:
                    # 特例：处理 Google Fonts 这种没有扩展名的CSS链接
                    if not ("googleapis.com/css" in url and context in ["html", "css"]):
                        continue
            except Exception:
                continue  # 解析失败的URL直接跳过

            self.processed_urls.add(cleaned_url)

            # 使用新的统一路径生成函数
            local_path, _ = self.get_resource_paths(cleaned_url, context)
            if not local_path:
                continue

            force_proxy = any(
                domain in cleaned_url for domain in self.force_proxy_domains
            )
            tasks.append(
                {
                    "url": cleaned_url,
                    "local_path": local_path,
                    "proxies": self.proxies,
                    "force_proxy": force_proxy,
                }
            )
        return tasks

    def localize_with_progress(self, progress_signal):
        # 1. 初始化队列
        self.text_content_queue.append(
            {"content": str(self.card_data), "context": "json"}
        )

        # 2. 循环处理队列，直到没有新的文本内容需要处理
        while self.text_content_queue and not self._stop_event.is_set():
            item = self.text_content_queue.pop(0)
            text_content, context = item["content"], item["context"]

            tasks = self.find_and_queue_urls(text_content, context)
            if not tasks:
                continue

            progress_signal.emit(
                f"在 {context} 上下文中发现 {len(tasks)} 个新URL，开始下载...", "info"
            )

            with ThreadPoolExecutor(max_workers=MAX_WORKERS) as executor:
                futures = {executor.submit(download_resource, task) for task in tasks}
                for future in as_completed(futures):
                    if self._stop_event.is_set():
                        executor.shutdown(
                            wait=False, cancel_futures=True
                        )  # 尝试取消未完成的任务
                        break  # 退出循环
                    result = future.result()
                    status, url = result["status"], result["url"]

                    method = result.get("method", "失败")
                    msg = f"[{method}] {url}"
                    if status == "failure":
                        msg += f" - {result['error']}"
                    progress_signal.emit(msg, status)

                    if status in ["success", "skipped"]:
                        # 使用新的统一路径生成函数获取Web路径
                        _, web_path = self.get_resource_paths(url, context)
                        if web_path:
                            self.successful_url_map[url] = web_path

                        # 如果下载的是文本文件，加入队列进行递归处理
                        content = result.get("content", b"")
                        if content and result["local_path"].suffix.lower() in [
                            ".css",
                            ".js",
                            ".html",
                            ".htm",
                        ]:
                            try:
                                new_context = (
                                    result["local_path"].suffix.lower().strip(".")
                                )
                                self.text_content_queue.append(
                                    {
                                        "content": content.decode("utf-8"),
                                        "context": new_context,
                                    }
                                )
                            except (UnicodeDecodeError, AttributeError):
                                pass

        # 3. 所有资源下载完毕后，进行最终的全局替换
        progress_signal.emit("所有资源下载完毕，正在替换路径...", "info")
        if self._stop_event.is_set():
            progress_signal.emit("本地化进程已终止，跳过路径替换。", "warning")
            return None

        final_data = self._replace_urls_recursive(
            self.card_data, self.successful_url_map
        )

        if self._stop_event.is_set():
            progress_signal.emit("路径替换过程中止。", "warning")
            return None
        else:
            progress_signal.emit("所有阶段处理完毕。", "success")
            return final_data

    def _replace_urls_recursive(self, data, url_map):
        if isinstance(data, dict):
            return {
                k: self._replace_urls_recursive(v, url_map) for k, v in data.items()
            }
        elif isinstance(data, list):
            return [self._replace_urls_recursive(i, url_map) for i in data]
        elif isinstance(data, str):
            for url, local_path in url_map.items():
                data = data.replace(url, local_path)
            return data

        return data

    def stop(self):
        self._stop_event.set()

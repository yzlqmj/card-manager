# -*- coding: utf-8 -*-
"""
角色卡本地化工具
cli.py - 命令行接口
"""

import argparse
import sys
import io
from pathlib import Path
import json

from card_utils import read_card_data, write_card_data
from localizer import Localizer
from config import load_settings as config_load

def main():
    """命令行主函数"""
    # --- 强制UTF-8输出 ---
    # 解决在非UTF-8终端（如Windows默认CMD）下被其他程序调用时乱码的问题
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')
    sys.stderr = io.TextIOWrapper(sys.stderr.buffer, encoding='utf-8')
    
    parser = argparse.ArgumentParser(description="角色卡本地化命令行工具")
    
    # 主要功能切换
    parser.add_argument(
        "card_path",
        type=str,
        help="要处理的角色卡PNG文件的绝对路径",
    )
    parser.add_argument(
        "--check",
        action="store_true",
        help="仅检查角色卡是否需要本地化，返回True或False",
    )

    # 可选参数，如果未提供则从config.json读取
    parser.add_argument(
        "--base-path",
        type=str,
        default=None,
        help="SillyTavern的public文件夹路径",
    )
    parser.add_argument(
        "--proxy",
        type=str,
        default=None,
        help="代理地址，例如: http://127.0.0.1:1233",
    )
    parser.add_argument(
        "--force-proxy-list",
        nargs='*',
        default=None,
        help="强制使用代理的域名列表，以空格分隔",
    )

    args = parser.parse_args()

    # --- 1. 加载配置 ---
    settings = config_load()
    
    # 如果命令行提供了参数，则覆盖配置文件中的设置
    base_path = args.base_path if args.base_path is not None else settings.get("base_path", "")
    proxy = args.proxy if args.proxy is not None else settings.get("proxy", "")
    force_proxy_list = args.force_proxy_list if args.force_proxy_list is not None else settings.get("force_proxy_list", [])

    # --- 2. 加载角色卡 ---
    input_filepath = Path(args.card_path)
    if not input_filepath.exists():
        print(f"错误: 角色卡文件不存在 -> {args.card_path}", file=sys.stderr)
        sys.exit(1)

    card_data, card_image = read_card_data(input_filepath)
    if not card_data or not card_image:
        print(f"错误: 无法从 {input_filepath.name} 读取角色卡数据。", file=sys.stderr)
        sys.exit(1)

    # --- 3. 执行请求的功能 ---
    
    # 临时Localizer实例，用于分析URL
    # 注意：这里的输出路径仅为临时占位符，因为我们只关心找到的URL数量
    temp_localizer = Localizer(
        card_data, Path("./temp_output"), proxy, force_proxy_list
    )
    
    # 初始将整个卡片数据作为JSON上下文进行分析
    temp_localizer.text_content_queue.append(
        {"content": json.dumps(card_data), "context": "json"}
    )

    all_urls_to_localize = set()
    
    # 循环分析，直到队列为空
    while temp_localizer.text_content_queue:
        item = temp_localizer.text_content_queue.pop(0)
        text_content, context = item["content"], item["context"]
        tasks = temp_localizer.find_and_queue_urls(text_content, context)
        for task in tasks:
            all_urls_to_localize.add(task["url"])
            # 在CLI模式下，我们不进行递归下载分析，只分析顶层
    
    needs_localization = len(all_urls_to_localize) > 0

    if args.check:
        print(needs_localization)
        sys.exit(0)

    # --- 4. 如果不是检查模式，则执行完整本地化 ---
    if not needs_localization:
        print("分析完成：未发现任何需要本地化的链接。")
        sys.exit(0)

    if not base_path or not Path(base_path).is_dir():
        print("错误: 请提供有效的SillyTavern public目录路径 (--base-path)", file=sys.stderr)
        sys.exit(1)

    print("开始本地化处理...")

    # 设置输出路径
    char_name = card_data.get("name", input_filepath.stem)
    safe_char_name = "".join(c for c in char_name if c.isalnum() or c in " _-").rstrip()
    resource_output_dir = Path(base_path) / "niko" / safe_char_name
    resource_output_dir.mkdir(parents=True, exist_ok=True)

    card_output_dir = input_filepath.parent / "本地化"
    card_output_dir.mkdir(parents=True, exist_ok=True)
    final_card_path = card_output_dir / input_filepath.name

    # 创建用于本地化的真实Localizer实例
    localizer = Localizer(card_data, resource_output_dir, proxy, force_proxy_list)

    # 定义一个模拟的信号类来适配 Localizer
    class CLISignals:
        def emit(self, message, level):
            print(f"[{level.upper()}] {message}")

    cli_signals = CLISignals()

    try:
        updated_card_data = localizer.localize_with_progress(cli_signals)

        if updated_card_data is None:
            print("本地化进程被终止或失败。", file=sys.stderr)
            sys.exit(1)

        success = write_card_data(card_image, updated_card_data, final_card_path)

        if success:
            print(f"本地化成功！新卡保存至: {final_card_path}")
            sys.exit(0)
        else:
            print("写入新的角色卡文件失败。", file=sys.stderr)
            sys.exit(1)

    except Exception as e:
        print(f"发生未预料的错误: {e}", file=sys.stderr)
        sys.exit(1)

if __name__ == "__main__":
    main()
# -*- coding: utf-8 -*-

"""
角色卡本地化工具
gui.py - 使用 PySide6 构建的图形用户界面
"""

import sys
import threading
from pathlib import Path
from PIL import Image

from PySide6.QtCore import Qt, Signal, QObject, QUrl
from PySide6.QtGui import QIcon, QAction, QDesktopServices
from PySide6.QtWidgets import (
    QApplication,
    QMainWindow,
    QWidget,
    QVBoxLayout,
    QHBoxLayout,
    QPushButton,
    QLineEdit,
    QLabel,
    QTextEdit,
    QFileDialog,
    QMessageBox,
    QProgressBar,
    QSplitter,
)
import pywinstyles

from card_utils import read_card_data, write_card_data
from localizer import Localizer
from config import load_settings as config_load, save_settings as config_save
from utils import resource_path


class WorkerSignals(QObject):
    progress = Signal(str, str)
    finished = Signal(bool, str)


class ApplicationWindow(QMainWindow):
    def __init__(self):
        super().__init__()
        self.setWindowTitle("角色卡资源本地化工具 v2.3")
        self.setGeometry(100, 100, 700, 660)
        self.setWindowIcon(QIcon(resource_path("ico.ico")))

        # --- 属性初始化 ---
        self.card_image = None
        self.card_data = None
        self.input_filepath = None
        self.current_localizer = None
        self.dark_stylesheet = ""
        self.final_card_path = None

        # --- UI 组件初始化 ---
        # 将所有UI组件的创建都提前，以防止在初始化过程中因错误日志而引发AttributeError
        central_widget = QWidget()
        self.setCentralWidget(central_widget)
        main_layout = QVBoxLayout(central_widget)

        # 顶部区域
        top_panel = QWidget()
        top_layout = QVBoxLayout(top_panel)
        path_layout = QHBoxLayout()
        self.base_path_label = QLabel("酒馆Public:")
        self.base_path_input = QLineEdit()
        self.base_path_button = QPushButton("浏览")
        path_layout.addWidget(self.base_path_label)
        path_layout.addWidget(self.base_path_input)
        path_layout.addWidget(self.base_path_button)
        top_layout.addLayout(path_layout)

        file_layout = QHBoxLayout()
        self.card_path_label = QLabel("角色卡文件:")
        self.filepath_input = QLineEdit()
        self.filepath_input.setReadOnly(True)
        self.load_button = QPushButton("浏览")
        file_layout.addWidget(self.card_path_label)
        file_layout.addWidget(self.filepath_input)
        file_layout.addWidget(self.load_button)
        top_layout.addLayout(file_layout)

        proxy_layout = QHBoxLayout()
        self.proxy_label = QLabel("代理地址:   ")
        self.proxy_input = QLineEdit()
        self.proxy_input.setPlaceholderText("例如: http://127.0.0.1:1233 或 socks5://127.0.0.1:1080")
        proxy_layout.addWidget(self.proxy_label)
        proxy_layout.addWidget(self.proxy_input)
        top_layout.addLayout(proxy_layout)

        # 中间可分割区域
        splitter = QSplitter(Qt.Horizontal)
        log_panel = QWidget()
        log_layout = QVBoxLayout(log_panel)
        log_label = QLabel("日志:")
        self.log_output = QTextEdit()
        self.log_output.setReadOnly(True)
        log_layout.addWidget(log_label)
        log_layout.addWidget(self.log_output)

        proxy_list_panel = QWidget()
        proxy_list_layout = QVBoxLayout(proxy_list_panel)
        proxy_list_label = QLabel("强制代理域名列表 (每行一个):")
        self.force_proxy_input = QTextEdit()
        proxy_list_layout.addWidget(proxy_list_label)
        proxy_list_layout.addWidget(self.force_proxy_input)

        splitter.addWidget(log_panel)
        splitter.addWidget(proxy_list_panel)
        splitter.setSizes([600, 300])

        # 底部区域
        bottom_panel = QWidget()
        bottom_layout = QVBoxLayout(bottom_panel)
        self.progress_bar = QProgressBar()
        self.progress_bar.setTextVisible(False)
        button_layout = QHBoxLayout()
        self.run_button = QPushButton("开始本地化")
        self.run_button.setEnabled(False)
        self.stop_button = QPushButton("终止本地化")
        self.stop_button.setEnabled(False)
        self.open_folder_button = QPushButton("打开文件夹")
        self.open_folder_button.setEnabled(False)
        button_layout.addWidget(self.run_button)
        button_layout.addWidget(self.stop_button)
        button_layout.addWidget(self.open_folder_button)
        bottom_layout.addWidget(self.progress_bar)
        bottom_layout.addLayout(button_layout)

        main_layout.addWidget(top_panel)
        main_layout.addWidget(splitter, 1)
        main_layout.addWidget(bottom_panel)

        # --- 连接信号和槽 ---
        self.base_path_button.clicked.connect(self.select_base_path)
        self.load_button.clicked.connect(self.load_card)
        self.run_button.clicked.connect(self.run_localization)
        self.stop_button.clicked.connect(self.stop_localization)
        self.open_folder_button.clicked.connect(self.open_output_folder)

        # --- 初始化完成后的操作 ---
        self.setup_theme_menu()
        self.load_settings()

    def setup_theme_menu(self):
        try:
            with open(resource_path("dark.qss"), "r", encoding="utf-8") as f:
                self.dark_stylesheet = f.read()
        except FileNotFoundError:
            self.log(f"主题文件 dark.qss 未找到。暗色主题将不可用。", "error")
            self.dark_stylesheet = ""

        menu_bar = self.menuBar()
        theme_menu = menu_bar.addMenu("主题")

        light_action = QAction("亮色", self)
        light_action.triggered.connect(lambda: self.switch_theme("light"))
        theme_menu.addAction(light_action)

        dark_action = QAction("暗色", self)
        dark_action.triggered.connect(lambda: self.switch_theme("dark"))
        theme_menu.addAction(dark_action)

    def switch_theme(self, theme):
        is_dark = theme == "dark"
        self.setStyleSheet(self.dark_stylesheet if is_dark else "")
        pywinstyles.apply_style(self, theme)
        self.save_settings()

    def load_settings(self):
        settings = config_load()
        self.base_path_input.setText(settings.get("base_path", ""))
        self.proxy_input.setText(settings.get("proxy", ""))
        self.force_proxy_input.setText("\n".join(settings.get("force_proxy_list", [])))

        if self.base_path_input.text():
            self.log(f"已加载保存的资源根目录: {self.base_path_input.text()}")

    def save_settings(self):
        force_proxy_list = [
            line.strip()
            for line in self.force_proxy_input.toPlainText().splitlines()
            if line.strip()
        ]
        current_theme = "dark" if self.styleSheet() else "light"
        settings = {
            "base_path": self.base_path_input.text(),
            "proxy": self.proxy_input.text(),
            "force_proxy_list": force_proxy_list,
            "theme": current_theme,
        }
        config_save(settings)
        self.log("设置已保存到 config.json。")

    def closeEvent(self, event):
        self.save_settings()
        event.accept()

    def select_base_path(self):
        directory = QFileDialog.getExistingDirectory(
            self, "请选择 SillyTavern 的 public 目录"
        )
        if directory:
            self.base_path_input.setText(str(Path(directory)))
            self.log(f"资源根目录已设置为: {self.base_path_input.text()}")

    def load_card(self):
        filepath, _ = QFileDialog.getOpenFileName(
            self, "选择角色卡", "", "PNG 文件 (*.png)"
        )
        if not filepath:
            return

        self.input_filepath = Path(filepath)
        self.filepath_input.setText(str(self.input_filepath))
        self.log(f"正在加载角色卡: {self.input_filepath.name}")

        card_data, card_image = read_card_data(self.input_filepath)

        if card_data and card_image:
            self.card_data = card_data
            self.card_image = card_image
            self.log("角色卡数据加载成功。", "success")
            self.run_button.setEnabled(True)

            # 统计待本地化的链接信息
            try:
                # 创建一个临时的Localizer实例来分析URL，不需要实际的输出目录和代理
                temp_localizer = Localizer(
                    self.card_data, Path("./temp_output"), None, []
                )
                # 模拟初始的文本内容队列，这里只处理card_data
                temp_localizer.text_content_queue.append(
                    {"content": str(self.card_data), "context": "json"}
                )

                all_urls_to_localize = set()
                force_proxy_urls = set()

                # 递归查找所有URL
                while temp_localizer.text_content_queue:
                    item = temp_localizer.text_content_queue.pop(0)
                    text_content, context = item["content"], item["context"]
                    tasks = temp_localizer.find_and_queue_urls(text_content, context)
                    for task in tasks:
                        url = task["url"]
                        all_urls_to_localize.add(url)
                        if task["force_proxy"]:
                            force_proxy_urls.add(url)

                        # 如果是可递归处理的文本文件，加入队列
                        if task["local_path"].suffix.lower() in [
                            ".css",
                            ".js",
                            ".html",
                            ".htm",
                        ]:
                            # 这里我们没有实际下载文件，所以无法获取内容
                            # 只能假设这些URL最终会被下载并包含更多URL
                            # 为了避免无限循环，这里不将它们的内容加入队列
                            # 实际的localize_with_progress会处理递归下载和解析
                            pass

                self.log("--- 角色卡链接分析报告 ---", "info")
                self.log(
                    f"总计发现 {len(all_urls_to_localize)} 个等待本地化的链接。", "info"
                )
                self.log(f"其中 {len(force_proxy_urls)} 个链接被强制代理。", "info")

                if all_urls_to_localize:
                    self.log("详细链接列表:", "info")
                    for url in sorted(list(all_urls_to_localize)):
                        proxy_status = " (强制代理)" if url in force_proxy_urls else ""
                        self.log(f"  {url}{proxy_status}", "info")
                else:
                    self.log("未发现任何需要本地化的链接。", "info")
                self.log("--- 报告结束 ---", "info")

            except Exception as e:
                self.log(f"分析链接时发生错误: {e}", "error")
        else:
            self.card_data = None
            self.card_image = None
            self.run_button.setEnabled(False)
            self.show_themed_message_box(
                "critical",
                "错误",
                "无法从此PNG文件中读取角色卡数据。\n请检查日志区域获取详细的调试信息。",
            )
            self.log("加载角色卡数据失败。正在输出原始元数据以供调试...", "error")
            self.debug_png_metadata(self.input_filepath)

    def debug_png_metadata(self, filepath):
        try:
            with Image.open(filepath) as img:
                if img.format != "PNG":
                    self.log(f"错误: '{filepath}' 不是一个PNG文件。", "error")
                    return

                self.log(f"--- 开始分析文件: {filepath} ---", "info")

                metadata = {}
                if hasattr(img, "info") and img.info:
                    metadata.update(img.info)
                if hasattr(img, "text") and img.text:
                    for k, v in img.text.items():
                        if k not in metadata:
                            metadata[k] = v

                if not metadata:
                    self.log("文件中未找到任何 'info' 或 'text' 元数据。", "info")
                    return

                self.log("\n发现以下元数据内容:", "info")
                for key, value in metadata.items():
                    self.log(f"  - 键 (Key): '{key}'", "info")
                    self.log(f"    值 (Value Preview): '{str(value)[:200]}...'", "info")

                self.log("\n--- 分析完成 ---", "info")
        except Exception as e:
            self.log(f"处理文件时发生未知错误: {e}", "error")

    def run_localization(self):
        if not self.card_data or not self.input_filepath:
            self.show_themed_message_box("critical", "错误", "没有加载任何角色卡。")
            return

        base_path = self.base_path_input.text().strip()
        if not base_path or not Path(base_path).is_dir():
            self.show_themed_message_box(
                "warning", "警告", "请先设置有效SillyTavern资源根目录 (public/)"
            )
            return

        char_name = self.card_data.get("name", self.input_filepath.stem)
        safe_char_name = "".join(
            c for c in char_name if c.isalnum() or c in " _-"
        ).rstrip()
        # 资源保存在SillyTavern的public目录中，以便Web服务器访问
        resource_output_dir = Path(base_path) / "niko" / safe_char_name
        resource_output_dir.mkdir(parents=True, exist_ok=True)

        # 最终的角色卡保存在原始卡片旁边的“本地化”子目录中
        card_output_dir = self.input_filepath.parent / "本地化"
        card_output_dir.mkdir(parents=True, exist_ok=True)
        self.final_card_path = card_output_dir / self.input_filepath.name

        self.set_ui_enabled(False)
        self.stop_button.setEnabled(True)  # 启用终止按钮
        self.progress_bar.setValue(0)

        proxy = self.proxy_input.text().strip() or None
        force_proxy_list = [
            line.strip()
            for line in self.force_proxy_input.toPlainText().splitlines()
            if line.strip()
        ]

        self.worker = threading.Thread(
            target=self.localization_thread,
            args=(
                self.card_data.copy(),
                resource_output_dir,
                self.final_card_path,
                proxy,
                force_proxy_list,
            ),
        )
        self.worker.start()

    def localization_thread(
        self, card_data, resource_output_dir, output_card_path, proxy, force_proxy_list
    ):
        signals = WorkerSignals()
        signals.progress.connect(self.update_log)
        signals.finished.connect(self.on_localization_finished)

        try:
            signals.progress.emit("开始本地化处理...", "info")
            self.current_localizer = Localizer(
                card_data, resource_output_dir, proxy, force_proxy_list
            )
            updated_card_data = self.current_localizer.localize_with_progress(
                signals.progress
            )

            # 如果 localize_with_progress 返回 None，说明进程被终止
            if updated_card_data is None:
                signals.finished.emit(False, "本地化进程被用户终止。")
                return

            success = write_card_data(
                self.card_image, updated_card_data, output_card_path
            )

            if success:
                signals.finished.emit(True, str(output_card_path))
            else:
                signals.finished.emit(False, "写入新的角色卡文件失败。")
        except Exception as e:
            signals.finished.emit(False, f"发生未预料的错误: {e}")

    def update_log(self, message, level):
        color_map = {
            "info": "#cccccc",  # 浅灰色，适合深色背景
            "success": "#28a745",  # 鲜绿色
            "skipped": "#6c757d",  # 灰色
            "error": "#dc3545",  # 红色
            "failure": "#dc3545",  # 红色
            "warning": "#ffc107",  # 黄色
        }
        color = color_map.get(level, "#cccccc")

        # 优化日志格式，增加级别显示
        level_text = level.upper()
        formatted_message = (
            f'<span style="color: {color};">'
            f"<b>[{level_text}]</b> {message}"
            f"</span>"
        )
        self.log_output.append(formatted_message)

        current_val = self.progress_bar.value()
        if current_val < 95:
            self.progress_bar.setValue(current_val + 1)

    def on_localization_finished(self, success, message):
        self.progress_bar.setValue(100)
        try:
            if success:
                self.final_card_path = Path(message)
                self.log(f"本地化成功！新卡保存至: {message}", "success")
                self.show_themed_message_box(
                    "information",
                    "成功",
                    f"本地化处理完成！\n新的角色卡已保存至:\n{message}",
                )
            else:
                self.log(f"本地化失败: {message}", "error")
                if "被用户终止" not in message:
                    self.show_themed_message_box(
                        "critical", "失败", f"本地化处理失败:\n{message}"
                    )
        finally:
            # 确保UI状态在任何情况下都会被重置
            self.set_ui_enabled(True)
            self.stop_button.setEnabled(False)
            self.current_localizer = None

    def set_ui_enabled(self, enabled):
        for widget in [
            self.load_button,
            self.run_button,
            self.base_path_button,
            self.base_path_input,
            self.proxy_input,
            self.force_proxy_input,
            self.filepath_input,
        ]:
            widget.setEnabled(enabled)

        # 单独处理“打开文件夹”按钮的逻辑
        if enabled:
            # 如果UI启用，则此按钮的可用性取决于是否存在有效路径
            self.open_folder_button.setEnabled(self.final_card_path is not None)
        else:
            # 如果UI禁用，则此按钮也禁用
            self.open_folder_button.setEnabled(False)

    def log(self, message, level="info"):
        from datetime import datetime

        timestamp = datetime.now().strftime("%H:%M:%S")
        self.update_log(f"[{timestamp}] {message}", level)

    def stop_localization(self):
        if hasattr(self, "current_localizer") and self.current_localizer:
            self.current_localizer.stop()
            self.log("本地化进程已请求终止。", "warning")
            self.stop_button.setEnabled(False)  # 立即禁用停止按钮

    def open_output_folder(self):
        if self.final_card_path and self.final_card_path.parent.exists():
            folder_path = self.final_card_path.parent
            if not QDesktopServices.openUrl(QUrl.fromLocalFile(str(folder_path))):
                self.log(f"无法自动打开文件夹: {folder_path}", "error")
                self.show_themed_message_box(
                    "warning", "错误", f"无法自动打开文件夹，请手动访问:\n{folder_path}"
                )
        else:
            self.show_themed_message_box(
                "information", "提示", "找不到输出目录，请先成功进行一次本地化。"
            )

    def show_themed_message_box(self, icon_type, title, text):
        msg_box = QMessageBox(self)
        msg_box.setWindowTitle(title)
        msg_box.setText(text)

        icon_map = {
            "information": QMessageBox.Information,
            "warning": QMessageBox.Warning,
            "critical": QMessageBox.Critical,
            "question": QMessageBox.Question,
        }
        msg_box.setIcon(icon_map.get(icon_type, QMessageBox.NoIcon))

        # Apply dark theme to the message box if the main window is dark
        if self.styleSheet():
            pywinstyles.apply_style(msg_box, "dark")

        msg_box.exec()


def main():
    # Enable High DPI support
    if hasattr(Qt, "AA_EnableHighDpiScaling"):
        QApplication.setAttribute(Qt.AA_EnableHighDpiScaling, True)
    if hasattr(Qt, "AA_UseHighDpiPixmaps"):
        QApplication.setAttribute(Qt.AA_UseHighDpiPixmaps, True)

    app = QApplication(sys.argv)
    # 图标已在 ApplicationWindow 中使用 resource_path 设置，此处无需重复设置
    # app.setWindowIcon(QIcon(resource_path("ico.ico")))
    window = ApplicationWindow()
    window.show()
    sys.exit(app.exec())

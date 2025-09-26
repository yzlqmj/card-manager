# -*- coding: utf-8 -*-

"""
角色卡本地化工具
main.py - 应用程序的主入口点
"""

import sys
from PySide6.QtWidgets import QApplication
from gui import ApplicationWindow
from config import load_settings
from utils import resource_path # 导入辅助函数
import pywinstyles


def main():
    """
    主函数，用于创建和运行PySide6应用程序。
    """
    settings = load_settings()
    theme = settings.get("theme", "light")

    # 创建应用程序实例
    app = QApplication(sys.argv)

    # 创建主窗口
    window = ApplicationWindow()

    # 在显示窗口前应用保存的主题
    if theme == "dark":
        window.setStyleSheet(window.dark_stylesheet)
        pywinstyles.apply_style(window, "dark")
    else:
        window.setStyleSheet("")
        pywinstyles.apply_style(window, "light")

    # 显示窗口
    window.show()
    # 进入应用程序的主事件循环
    sys.exit(app.exec())


if __name__ == "__main__":
    main()

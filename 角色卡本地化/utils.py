# -*- coding: utf-8 -*-

"""
辅助工具函数
"""

import sys
import os


def resource_path(relative_path):
    """
    获取资源的绝对路径，无论是从源代码运行还是从 PyInstaller 打包的 exe 运行。
    """
    try:
        # PyInstaller 创建一个临时文件夹，并将路径存储在 _MEIPASS 中
        base_path = sys._MEIPASS
    except Exception:
        # 如果不是通过 PyInstaller 运行，则使用常规路径
        base_path = os.path.abspath(".")

    return os.path.join(base_path, relative_path)
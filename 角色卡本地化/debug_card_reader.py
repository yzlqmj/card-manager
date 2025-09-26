# -*- coding: utf-8 -*-

"""
角色卡PNG元数据调试脚本
用法: python debug_card_reader.py "你的角色卡文件路径.png"
"""

import sys
from PIL import Image


def debug_png_metadata(filepath):
    """
    打开一个PNG文件并打印出其所有的文本元数据块。
    """
    try:
        with Image.open(filepath) as img:
            if img.format != "PNG":
                print(f"错误: '{filepath}' 不是一个PNG文件。")
                return

            print(f"--- 正在分析文件: {filepath} ---")

            if not hasattr(img, "info") or not img.info:
                print("文件中未找到 'info' 元数据字典。")
                # Pillow 早期版本可能将文本块存储在 .text 属性中
                if hasattr(img, "text") and img.text:
                    print("\n发现 'text' 属性，内容如下:")
                    for key, value in img.text.items():
                        print(f"  - 键 (Key): '{key}'")
                        # 尝试打印值的开头部分，以防内容过长
                        print(f"    值 (Value Preview): '{str(value)[:200]}...'")
                else:
                    print("也未发现 'text' 属性。该文件可能不包含任何文本元数据。")
                return

            print("\n发现 'info' 元数据字典，内容如下:")
            for key, value in img.info.items():
                print(f"  - 键 (Key): '{key}'")
                print(f"    值 (Value Preview): '{str(value)[:200]}...'")

            print("\n--- 分析完成 ---")

    except FileNotFoundError:
        print(f"错误: 文件未找到 -> {filepath}")
    except Exception as e:
        print(f"处理文件时发生未知错误: {e}")


if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("错误: 请提供一个PNG文件路径作为参数。")
        print('用法: python debug_card_reader.py "你的角色卡文件路径.png"')
    else:
        filepath = sys.argv[1]
        debug_png_metadata(filepath)

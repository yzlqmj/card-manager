# -*- coding: utf-8 -*-

"""
角色卡本地化工具
build.py - 使用 PyInstaller 的自动化打包脚本
"""

import os
import shutil
import subprocess
import sys

# --- 配置 ---
APP_NAME = "角色卡本地化"
CLI_APP_NAME = "cli"
MAIN_SCRIPT = "main.py"
CLI_SCRIPT = "cli.py"
ICON_FILE = "ico.ico"
OUTPUT_DIR = "dist"
BUILD_DIR = "build"

# 需要包含到打包中的额外文件或目录
# 格式: (源路径, 在包内的目标路径)
# 例如: ('data/images', 'images')
DATA_FILES = [
    ("ico.ico", "."),
    ("dark.qss", "."),
]

# PyInstaller 可能无法自动找到的隐藏导入
HIDDEN_IMPORTS = [
    "pywinstyles",
    "utils",  # 确保我们的辅助模块被包含
    "socks",  # cli.py 可能会用到
]


def clean():
    """清理旧的打包文件和目录"""
    print("--- 正在清理旧文件 ---")
    for path in [BUILD_DIR, OUTPUT_DIR, f"{APP_NAME}.spec", f"{CLI_APP_NAME}.spec"]:
        if os.path.exists(path):
            try:
                if os.path.isdir(path):
                    shutil.rmtree(path)
                    print(f"已删除目录: {path}")
                else:
                    os.remove(path)
                    print(f"已删除文件: {path}")
            except OSError as e:
                print(f"清理 {path} 时出错: {e}", file=sys.stderr)
                sys.exit(1)
    print("--- 清理完成 ---\n")


def run_pyinstaller(script, name, is_gui):
    """
    通用打包函数
    :param script: 入口脚本 (e.g., 'main.py')
    :param name: 输出的应用名称
    :param is_gui: 是否是GUI应用 (True for --windowed, False for --console)
    """
    pyinstaller_command = [
        "uv", "run", "pyinstaller",
        "--noconfirm",
        "--onedir",
        f"--name={name}",
    ]

    if is_gui:
        pyinstaller_command.append("--windowed")
        pyinstaller_command.append(f"--icon={ICON_FILE}")
    else:
        pyinstaller_command.append("--console")

    for src, dst in DATA_FILES:
        pyinstaller_command.append(f"--add-data={src}{os.pathsep}{dst}")

    for lib in HIDDEN_IMPORTS:
        pyinstaller_command.append(f"--hidden-import={lib}")

    pyinstaller_command.append(script)

    print(f"--- 开始打包 {name} ---")
    print(f"执行命令: {' '.join(pyinstaller_command)}")

    try:
        process = subprocess.Popen(
            pyinstaller_command,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            encoding=None,
            errors='ignore'
        )
        for line in iter(process.stdout.readline, ''):
            print(line.strip())
        process.wait()

        if process.returncode != 0:
            print(f"\n!!! 打包 {name} 失败，返回代码: {process.returncode} !!!", file=sys.stderr)
            return False
        
        print(f"\n--- 打包 {name} 成功 ---")
        return True

    except FileNotFoundError:
        print("\n错误: 'uv' 命令未找到。", file=sys.stderr)
        print("请确保已安装 uv 并将其添加到系统 PATH。", file=sys.stderr)
        return False
    except Exception as e:
        print(f"\n打包 {name} 过程中发生未知错误: {e}", file=sys.stderr)
        return False

def build():
    """执行所有打包任务"""
    clean()
    
    # 1. 打包GUI应用
    gui_success = run_pyinstaller(MAIN_SCRIPT, APP_NAME, is_gui=True)
    if not gui_success:
        sys.exit(1)
        
    # 2. 打包CLI应用
    cli_success = run_pyinstaller(CLI_SCRIPT, CLI_APP_NAME, is_gui=False)
    if not cli_success:
        sys.exit(1)

    print("\n======================================")
    print("所有打包任务完成!")
    print(f"文件位于: {os.path.abspath(OUTPUT_DIR)}")
    print("======================================")


if __name__ == "__main__":
    build()
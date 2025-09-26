# -*- coding: utf-8 -*-
"""
角色卡本地化工具
config.py - 配置文件读写模块
"""

import json
from pathlib import Path

CONFIG_FILE = Path("config.json")

# 默认的强制代理域名列表
DEFAULT_FORCE_PROXY_DOMAINS = [
    "gitgud.io",
    "raw.githubusercontent.com",
    "cdn.jsdelivr.net",
    "github.com",
    "fonts.googleapis.com",
    "files.catbox.moe",
]

DEFAULT_SETTINGS = {
    "base_path": "",
    "proxy": "",
    "force_proxy_list": DEFAULT_FORCE_PROXY_DOMAINS,
}


def load_settings():
    """从 config.json 加载设置"""
    if not CONFIG_FILE.exists():
        return DEFAULT_SETTINGS
    try:
        with open(CONFIG_FILE, "r", encoding="utf-8") as f:
            settings = json.load(f)
            # 确保所有默认键都存在
            for key, value in DEFAULT_SETTINGS.items():
                settings.setdefault(key, value)
            return settings
    except (json.JSONDecodeError, IOError):
        return DEFAULT_SETTINGS


def save_settings(settings):
    """将设置保存到 config.json"""
    try:
        with open(CONFIG_FILE, "w", encoding="utf-8") as f:
            json.dump(settings, f, indent=4, ensure_ascii=False)
    except IOError:
        # 在GUI中可以处理保存失败的情况
        pass

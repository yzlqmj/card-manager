# -*- coding: utf-8 -*-

"""
角色卡本地化工具
card_utils.py - 用于从PNG文件读取和写入角色卡数据的工具函数
"""

import json
import base64
from PIL import Image, PngImagePlugin


def read_card_data(filepath):
    """
    从PNG文件中读取角色卡数据。
    此版本通过检查 'info' 和 'text' 两个属性来提高兼容性。

    Args:
        filepath (str): 角色卡PNG文件的路径。

    Returns:
        dict: 以字典形式返回的角色卡数据，如果未找到数据则返回None。
        Image: Pillow的Image对象，用于后续写入。
    """
    try:
        with Image.open(filepath) as img:
            if img.format != "PNG":
                return None, None

            # Pillow可能会将文本块存储在 info 或 text 属性中
            # 我们将它们合并到一个字典中进行检查，以 info 的优先级更高
            metadata = {}
            if hasattr(img, "info") and img.info:
                metadata.update(img.info)
            if hasattr(img, "text") and img.text:
                # .text 的键通常是小写的，我们将其与 .info 合并
                # 如果有同名键，.info 的值将被保留
                for k, v in img.text.items():
                    if k not in metadata:
                        metadata[k] = v

            if not metadata:
                return None, None

            # 将所有键转换为小写以便进行不区分大小写的比较
            info_lower = {k.lower(): v for k, v in metadata.items()}

            # V3版本的角色卡数据 (ccv3) 优先
            key_to_check = "ccv3"
            if key_to_check in info_lower:
                raw_data = info_lower[key_to_check]
                decoded_data = base64.b64decode(raw_data).decode("utf-8")
                return json.loads(decoded_data), img.copy()

            # 回退到V2版本的角色卡数据 (chara)
            key_to_check = "chara"
            if key_to_check in info_lower:
                raw_data = info_lower[key_to_check]
                decoded_data = base64.b64decode(raw_data).decode("utf-8")
                return json.loads(decoded_data), img.copy()

            return None, None
    except Exception as e:
        print(f"从 {filepath} 读取角色卡数据时出错: {e}")
        return None, None


def write_card_data(image, card_data, output_path):
    """
    将角色卡数据写入一个新的PNG文件。

    Args:
        image (Image): 原始的Pillow Image对象。
        card_data (dict): 要写入的角色卡数据字典。
        output_path (str): 新PNG文件的保存路径。
    """
    try:
        png_info = PngImagePlugin.PngInfo()

        v2_card_data = card_data.copy()
        if "spec" in v2_card_data:
            del v2_card_data["spec"]
        if "spec_version" in v2_card_data:
            del v2_card_data["spec_version"]
        v2_data_str = json.dumps(v2_card_data, ensure_ascii=False)
        v2_base64 = base64.b64encode(v2_data_str.encode("utf-8")).decode("utf-8")
        png_info.add_text("chara", v2_base64)

        v3_card_data = card_data.copy()
        v3_card_data["spec"] = "chara_card_v3"
        v3_card_data["spec_version"] = "3.0"
        v3_data_str = json.dumps(v3_card_data, ensure_ascii=False)
        v3_base64 = base64.b64encode(v3_data_str.encode("utf-8")).decode("utf-8")
        png_info.add_text("ccv3", v3_base64)

        image.save(output_path, "PNG", pnginfo=png_info)
        return True
    except Exception as e:
        print(f"向 {output_path} 写入角色卡数据时出错: {e}")
        return False

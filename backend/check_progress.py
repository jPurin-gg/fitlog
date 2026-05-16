import json
import re

def is_japanese(text):
    if not text:
        return False
    return bool(re.search(r'[\u3040-\u309F\u30A0-\u30FF\u4E00-\u9FAF]', text))

def main():
    try:
        with open('tmpkin_jp.json', 'r', encoding='utf-8') as f:
            data = json.load(f)
    except Exception as e:
        print(f"Error loading JSON: {e}")
        return

    translated_count = 0
    total = len(data)

    for item in data:
        # 名前か説明の最初の要素に日本語が含まれていれば翻訳済みとみなす
        name_jp = is_japanese(item.get('name', ''))
        inst_jp = False
        instructions = item.get('instructions', [])
        if instructions and len(instructions) > 0:
            inst_jp = is_japanese(instructions[0])
            
        if name_jp or inst_jp:
            translated_count += 1

    print(f"Total items: {total}")
    print(f"Translated: {translated_count}")
    print(f"Remaining: {total - translated_count}")

if __name__ == '__main__':
    main()

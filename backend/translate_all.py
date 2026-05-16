import json
import time
import re
from deep_translator import GoogleTranslator

def is_japanese(text):
    if not text:
        return False
    # ひらがな、カタカナ、漢字のいずれかが含まれていれば日本語と判定
    return bool(re.search(r'[\u3040-\u309F\u30A0-\u30FF\u4E00-\u9FAF]', text))

def translate_text(text):
    if not text:
        return text
    if is_japanese(text):
        return text # すでに日本語なら翻訳しない
        
    for _ in range(3):
        try:
            return GoogleTranslator(source='en', target='ja').translate(text)
        except Exception as e:
            time.sleep(2)
    return text

def process_item(item):
    if 'name' in item and isinstance(item['name'], str):
        if not is_japanese(item['name']):
            item['name'] = translate_text(item['name'])
    
    if 'instructions' in item and isinstance(item['instructions'], list):
        for i, inst in enumerate(item['instructions']):
            if not is_japanese(inst):
                item['instructions'][i] = translate_text(inst)
        
    return item

def main():
    try:
        with open('tmpkin_jp.json', 'r', encoding='utf-8') as f:
            data = json.load(f)
    except FileNotFoundError:
        print("Error: tmpkin_jp.json not found.")
        return
        
    print(f"Resuming translation. Total items: {len(data)}")
    
    translated_this_run = 0
    for i, item in enumerate(data):
        name_jp = is_japanese(item.get('name', ''))
        
        if name_jp:
            continue # すでに翻訳済みはスキップ
            
        print(f"Translating [{i+1}/{len(data)}]: {item.get('name', 'Unknown')}")
        process_item(item)
        translated_this_run += 1
        
        # 20件ごとにこまめにセーブ
        if translated_this_run % 20 == 0:
            with open('tmpkin_jp.json', 'w', encoding='utf-8') as f:
                json.dump(data, f, ensure_ascii=False, indent=2)
            print("--- Partial save completed ---")
            
    with open('tmpkin_jp.json', 'w', encoding='utf-8') as f:
        json.dump(data, f, ensure_ascii=False, indent=2)
        
    print("All translation completed successfully!")

if __name__ == '__main__':
    main()

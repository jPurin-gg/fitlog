import json
import os

def main():
    try:
        with open('tmpkin_jp.json', 'r', encoding='utf-8') as f:
            data = json.load(f)
            
        with open('update_translations.sql', 'w', encoding='utf-8') as out:
            for item in data:
                name = item.get('name', '').replace("'", "''")
                inst_json = json.dumps(item.get('instructions', []), ensure_ascii=False).replace("'", "''")
                
                cat = item.get('category')
                cat_val = f"'{cat.replace('\"', '')}'" if cat else "NULL"
                
                eq = item.get('equipment')
                eq_val = f"'{eq.replace('\"', '')}'" if eq else "NULL"
                
                _id = item['id'].replace("'", "''")
                
                sql = f"UPDATE exercises SET name = '{name}', instructions = '{inst_json}'::jsonb, category = {cat_val}, equipment = {eq_val} WHERE id = '{_id}';\n"
                out.write(sql)
                
        print("SQL script generated successfully.")
    except Exception as e:
        print(f"Error: {e}")

if __name__ == '__main__':
    main()

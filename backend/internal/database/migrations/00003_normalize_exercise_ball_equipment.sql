-- 種目カタログで未翻訳のまま投入されていた器具名を、他の器具と同じ日本語表記にそろえる。
-- seed（tmpkin_jp.json）も同じ表記へ変更済みで、投入済みのDBだけがこの更新の対象になる。
UPDATE exercises SET equipment = 'バランスボール' WHERE equipment = 'exercise ball';

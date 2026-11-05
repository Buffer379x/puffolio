CREATE TABLE tags_new(id TEXT PRIMARY KEY,collection_id TEXT NOT NULL,field_id TEXT NOT NULL,name TEXT NOT NULL,normalized_name TEXT NOT NULL,palette_id TEXT NOT NULL CHECK(palette_id IN ('lime','pistachio','olive','moss','green','mint','jade','teal','aqua','cyan','sky','ice','blue','cornflower','periwinkle','indigo','lavender','violet','orchid','pink','magenta','rose','raspberry','red','coral','apricot','orange','tangerine','amber','honey','sand','slate')),version INTEGER NOT NULL DEFAULT 1,UNIQUE(collection_id,field_id,normalized_name),UNIQUE(collection_id,field_id,id),FOREIGN KEY(collection_id,field_id) REFERENCES field_definitions(collection_id,id) ON DELETE CASCADE);
INSERT INTO tags_new(id,collection_id,field_id,name,normalized_name,palette_id,version) SELECT id,collection_id,field_id,name,normalized_name,palette_id,version FROM tags;
DROP TABLE tags;
ALTER TABLE tags_new RENAME TO tags;
CREATE TRIGGER tag_field_type BEFORE INSERT ON tags WHEN (SELECT type FROM field_definitions WHERE id=NEW.field_id AND collection_id=NEW.collection_id)!='tags' BEGIN SELECT RAISE(ABORT,'tag field required'); END;
PRAGMA user_version=2;

#!/bin/bash

sqlite3 -header -column "data/one-api.db" "SELECT id, name, \"group\", status, substr(key, 1, 8) AS key_prefix FROM tokens WHERE user_id = 1 ORDER BY id; SELECT key, value FROM options WHERE key IN ('UserUsableGroups', 'GroupRatio');"

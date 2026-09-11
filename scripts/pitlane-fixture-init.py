#!/usr/bin/env python3
"""Prepare an isolated installation using copies of local display metadata.
Run inside pitlane-fixture after its first boot. Never uses the real database.
"""
import pathlib
import shutil
import sqlite3

repo = pathlib.Path(__file__).resolve().parents[1]
install = pathlib.Path('/tmp/pitlane-fixture/install')
install.mkdir(parents=True, exist_ok=True)
for relative in ['content/tracks/drift/ui', 'content/tracks/drift/data', 'content/cars/abarth500_s1/ui',
                 'content/cars/abarth500_s1/skins/dark_blue_blue', 'content/weather/3_clear']:
    shutil.copytree(repo / 'assetocorsa' / relative, install / relative, dirs_exist_ok=True)
for relative in ['content/tracks/drift/models.ini', 'content/tracks/drift/map.png', 'content/cars/abarth500_s1/data.acd']:
    source = repo / 'assetocorsa' / relative
    if source.exists():
        shutil.copy2(source, install / relative)
(install / 'server').mkdir(exist_ok=True)
shutil.copy2(repo / 'scripts/pitlane-fixture-server.py', install / 'server/acServer')
(install / 'server/acServer').chmod(0o755)
connection = sqlite3.connect('/tmp/pitlane-data/servermanager/smdata.db')
connection.execute("UPDATE user_config SET install_path=?,cfg_filled=1,mod_filled=1,server_engine='kunos',max_clients=8,name='Pitlane test club',register_to_lobby=0", (str(install),))
connection.execute("UPDATE server_instance SET name='Protocol fixture · not a game server'")
connection.commit()
connection.close()
print('Prepared only /tmp/pitlane-fixture and the temporary Pitlane DB. Rebuild the cache through the API.')

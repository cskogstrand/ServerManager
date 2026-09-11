#!/usr/bin/env python3
"""Disposable AC UDP protocol fixture. This is NOT a game server.

Runs only from /tmp/pitlane-data or /tmp/pitlane-fixture. It supplies labelled
telemetry through the real UDP adapter for browser, map and ownership tests.
"""
import configparser
import json
import math
import pathlib
import socket
import struct
import time

cwd = pathlib.Path.cwd().resolve()
if not any(cwd.is_relative_to(pathlib.Path(root)) for root in ('/tmp/pitlane-data', '/tmp/pitlane-fixture')):
    raise SystemExit('Fixture refuses to run outside a disposable Pitlane directory')
config = configparser.ConfigParser(strict=False, interpolation=None)
config.read(cwd / 'cfg/server_cfg.ini')
server = config['SERVER']
entries = configparser.ConfigParser(strict=False, interpolation=None)
entries.read(cwd / 'cfg/entry_list.ini')
car = entries['CAR_0'].get('MODEL', 'abarth500_s1')
skin = entries['CAR_0'].get('SKIN', 'dark_blue_blue')
track = server.get('TRACK', 'drift')
layout = server.get('CONFIG_TRACK', '')
port = int(server.get('UDP_PLUGIN_LOCAL_PORT', '5000'))
host, destination_port = server.get('UDP_PLUGIN_ADDRESS', '127.0.0.1:5001').split(':')
sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
sock.bind(('127.0.0.1', port))
sock.setblocking(False)
destination = (host, int(destination_port))

def text(value, utf32=False):
    return bytes([len(value)]) + value.encode('utf-32-le' if utf32 else 'utf-8')

def send(packet):
    sock.sendto(packet, destination)

def phase(kind=50):
    send(bytes([kind, 4, 0, 0, 2]) + text('Pitlane protocol fixture', True) + text(track) + text(layout)
         + text('Practice - fixture') + struct.pack('<BHHHBB', 1, 60, 0, 0, 20, 27) + text('3_clear') + struct.pack('<i', 0))

time.sleep(1)
send(bytes([56, 4]))
phase()
for i in range(2):
    send(bytes([51]) + text(f'Fixture driver {i + 1}', True) + text(f'pitlane-fixture-{i + 1}', True)
         + bytes([i]) + text(car) + text(skin))
started = time.monotonic()
last_lap = -1
result_sent = False
while True:
    elapsed = time.monotonic() - started
    for i in range(2):
        angle = elapsed / 20 + i * math.pi
        send(bytes([53, i]) + struct.pack('<ffffffBHf', math.cos(angle) * 70, 0, math.sin(angle) * 60,
                                         0, 0, 0, 2, 2100 + i * 450, (elapsed / 60 + i / 2) % 1))
    lap = int(elapsed / 12)
    if lap != last_lap and lap > 0:
        for i in range(2):
            send(bytes([73, i]) + struct.pack('<IB', 61234 + i * 1234, 0))
        last_lap = lap
    if elapsed > 30 and not result_sent:
        result = cwd / 'results/pitlane-fixture-result.json'
        result.parent.mkdir(exist_ok=True)
        result.write_text(json.dumps({'Type': 'PRACTICE', 'TrackName': track, 'Fixture': True,
            'Result': [{'DriverName': 'Fixture driver 1', 'DriverGuid': 'pitlane-fixture-1', 'BestLap': 61234}]}))
        send(bytes([55]) + text(str(result), True))
        result_sent = True
    try:
        packet, _ = sock.recvfrom(2048)
        if packet and packet[0] in (204, 207, 208):
            phase(59 if packet[0] == 204 else 50)
    except BlockingIOError:
        pass
    time.sleep(0.2)

#!/usr/bin/env python3
"""Local-only video fixture. Run after creating the disposable pitlane-fixture container.
Serves a real MP4 player and continuous MPEG-TS from ffmpeg, with no external camera.
"""
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
import subprocess

folder=Path('/tmp/pitlane-video')
folder.mkdir(exist_ok=True)
video=folder/'fixture.mp4'
if not video.exists():
    subprocess.run(['docker','exec','pitlane-fixture','ffmpeg','-v','error','-f','lavfi','-i','testsrc2=size=960x540:rate=24','-t','8','-c:v','libx264','-preset','ultrafast','-pix_fmt','yuv420p','-movflags','+faststart','-y','/tmp/pitlane-fixture.mp4'],check=True)
    subprocess.run(['docker','cp','pitlane-fixture:/tmp/pitlane-fixture.mp4',str(video)],check=True)
else:
    subprocess.run(['docker','cp',str(video),'pitlane-fixture:/tmp/pitlane-fixture.mp4'],check=True)
class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path=='/stream.ts':
            self.send_response(200);self.send_header('Content-Type','video/mp2t');self.end_headers()
            process=subprocess.Popen(['docker','exec','pitlane-fixture','ffmpeg','-v','error','-re','-stream_loop','-1','-i','/tmp/pitlane-fixture.mp4','-c','copy','-f','mpegts','pipe:1'],stdout=subprocess.PIPE)
            try:
                while chunk:=process.stdout.read(32768):self.wfile.write(chunk)
            except (BrokenPipeError,ConnectionResetError):pass
            finally:process.terminate();process.wait()
            return
        if self.path=='/fixture.mp4':data=video.read_bytes();kind='video/mp4'
        elif self.path=='/status':data=b'{"ready":true}';kind='application/json'
        elif self.path.startswith('/player'):
            data=b'''<!doctype html><html><meta name="viewport" content="width=device-width"><style>html,body{margin:0;height:100%;background:#111;color:#fff;font:16px sans-serif}video{width:100%;height:100%;object-fit:contain}b{position:absolute;top:16px;left:16px;background:#000a;padding:8px}</style><video src="/fixture.mp4" autoplay muted loop playsinline controls></video><b>Protocol fixture - test video</b></html>''';kind='text/html'
        else:self.send_error(404);return
        self.send_response(200);self.send_header('Content-Type',kind);self.send_header('Content-Length',str(len(data)));self.end_headers();self.wfile.write(data)
ThreadingHTTPServer(('0.0.0.0',8787),Handler).serve_forever()

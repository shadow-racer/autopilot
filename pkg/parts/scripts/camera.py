#!/usr/bin/env python3
# -*- coding: utf-8 -*-

import io
import time
import logging
import traceback
import socketserver
import argparse
import picamera
import json
import base64
import sys

from threading import Condition
from http import server
from datetime import datetime
from urllib.parse import parse_qs


def timestamp():
    return int(datetime.utcnow().timestamp() * 1000000)


class StreamingOutput(object):
    def __init__(self):
        self.frame = None
        self.buffer = io.BytesIO()
        self.condition = Condition()
        self.recording = False
        self.batch = timestamp()
        self.framecounter = 0

    def start_recording(self, ts=0):
        if not self.recording:
            if ts == 0:
                self.batch = timestamp()
            else:
                self.batch = ts
            self.recording = True
            self.framecounter = 0

    def stop_recording(self):
        if self.recording:
            self.recording = False

    def write(self, buf):
        if buf.startswith(b'\xff\xd8'):
            # New frame, copy the existing buffer's content and notify all
            # clients it's available
            self.buffer.truncate()
            with self.condition:
                self.frame = self.buffer.getvalue()
                self.condition.notify_all()
            self.buffer.seek(0)

            if self.recording:
                sys.stdout.buffer.write(buf)
                sys.stdout.flush()

                self.framecounter = self.framecounter + 1

        return self.buffer.write(buf)


class StreamingHandler(server.BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == '/stream.mjpg':
            self.send_response(200)
            self.send_header('Age', 0)
            self.send_header('Cache-Control', 'no-cache, private')
            self.send_header('Pragma', 'no-cache')
            self.send_header(
                'Content-Type', 'multipart/x-mixed-replace; boundary=FRAME')
            self.end_headers()

            try:
                while True:
                    with output.condition:
                        output.condition.wait()
                        frame = output.frame

                    self.wfile.write(b'--FRAME\r\n')
                    self.send_header('Content-Type', 'image/jpeg')
                    self.send_header('Content-Length', len(frame))
                    self.end_headers()
                    self.wfile.write(frame)
                    self.wfile.write(b'\r\n')

            except Exception as e:
                logging.warning(
                    'Removed streaming client %s: %s',
                    self.client_address, str(e))

        elif self.path.startswith('/start'):
            if "?ts" in self.path:
                output.start_recording(int(parse_qs(self.path[7:])['ts'][0]))
            else:
                output.start_recording()
            self.send_response(200)
            self.end_headers()

        elif self.path == '/stop':
            output.stop_recording()
            self.send_response(200)
            self.end_headers()

        elif self.path == '/':
            self.send_response(301)
            self.send_header('Location', '/stream.mjpg')
            self.end_headers()

        else:
            self.send_error(404)
            self.end_headers()


class StreamingServer(socketserver.ThreadingMixIn, server.HTTPServer):
    allow_reuse_address = True
    daemon_threads = True


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('-p', '--port', dest='port', type=int, default='3001')
    parser.add_argument('-r', '--resolution',
                        dest='resolution', default='1024x768')
    parser.add_argument('-f', '--fps', dest='fps', type=int, default='30')

    args = parser.parse_args()

    with picamera.PiCamera(resolution=args.resolution, framerate=args.fps) as camera:
        output = StreamingOutput()

        camera.start_preview()
        time.sleep(2)
        camera.rotation = 180
        camera.start_recording(output, format='mjpeg')

        try:
            address = ('', args.port)
            server = StreamingServer(address, StreamingHandler)
            server.serve_forever()
        except KeyboardInterrupt:
            pass
        except Exception as e:
            traceback.print_exc()
        finally:
            camera.stop_recording()

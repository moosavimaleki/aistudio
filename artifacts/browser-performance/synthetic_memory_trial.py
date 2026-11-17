import json, os, pathlib, statistics, subprocess, tempfile, time
import urllib.request
from http.server import BaseHTTPRequestHandler, HTTPServer
from threading import Thread
import websocket

class Page(BaseHTTPRequestHandler):
    def log_message(self, *args): pass
    def do_GET(self):
        self.send_response(200)
        self.send_header('Content-Type', 'text/html')
        self.end_headers()
        self.wfile.write(b'<html><body><input id="composer"><script>for(let i=0;i<1000;i++){let p=document.createElement("p");p.textContent="Synthetic conversation message "+i;document.body.append(p)}</script></body></html>')

mode = os.environ['TRIAL_MODE']
server = HTTPServer(('127.0.0.1',0), Page)
Thread(target=server.serve_forever,daemon=True).start()
root = tempfile.TemporaryDirectory()
ext = pathlib.Path(root.name)/'extension'
ext.mkdir()
(ext/'manifest.json').write_text(json.dumps({'manifest_version':3,'name':'Synthetic trial','version':'1.0','content_scripts':[{'matches':['http://127.0.0.1/*'],'js':['test.js']}]}))
(ext/'test.js').write_text('document.documentElement.dataset.trial="ready";')
processes=[]
def spawn(args):
    p=subprocess.Popen(args,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
    processes.append(p)
    return p
try:
    if mode=='headed':
        spawn(['Xvfb',':88','-screen','0','1280x900x24','-nolisten','tcp','-ac'])
        os.environ['DISPLAY']=':88'
        time.sleep(2)
        spawn(['fluxbox','-display',':88','-no-slit','-no-toolbar'])
        spawn(['x11vnc','-display',':88','-nopw','-forever','-shared','-rfbport','5900'])
        spawn(['/opt/venv/bin/websockify','7900','127.0.0.1:5900'])
    flags=['/opt/cloakbrowser/chrome','--no-sandbox','--disable-gpu','--no-first-run','--no-default-browser-check','--disable-background-networking','--disable-component-update','--disable-sync','--renderer-process-limit=2','--num-raster-threads=1','--window-size=1280,900','--remote-debugging-port=9229','--user-data-dir='+root.name+'/profile','--load-extension='+str(ext),'--disable-extensions-except='+str(ext)]
    if mode=='headless': flags+=['--headless=new']
    spawn(flags+[f'http://127.0.0.1:{server.server_port}/'])
    ready=False
    for attempt in range(40):
        try:
            tabs=json.load(urllib.request.urlopen('http://127.0.0.1:9229/json/list',timeout=2))
            tab=next(t for t in tabs if t['type']=='page')
            ws=websocket.create_connection(tab['webSocketDebuggerUrl'],suppress_origin=True,timeout=3)
            ws.send(json.dumps({'id':1,'method':'Runtime.evaluate','params':{'expression':'document.documentElement.dataset.trial === "ready" && document.querySelectorAll("p").length === 1000','returnByValue':True}}))
            while True:
                event=json.loads(ws.recv())
                if event.get('id')==1:
                    ready=event.get('result',{}).get('result',{}).get('value') is True
                    break
            ws.close()
            if ready: break
        except Exception: pass
        time.sleep(1)
    if not ready: raise RuntimeError('synthetic page/extension not ready')
    time.sleep(5)
    samples=[]
    for _ in range(10):
        current=int(pathlib.Path('/sys/fs/cgroup/memory.current').read_text())
        stat=dict(line.split() for line in pathlib.Path('/sys/fs/cgroup/memory.stat').read_text().splitlines())
        samples.append((current-int(stat.get('inactive_file',0)))/1048576)
        time.sleep(1)
    print(json.dumps({'mode':mode,'page_ready':ready,'working_set_mib_mean':round(statistics.mean(samples),2),'min':round(min(samples),2),'max':round(max(samples),2),'samples':len(samples)}),flush=True)
finally:
    for p in reversed(processes):
        p.terminate()
    for p in reversed(processes):
        try:p.wait(timeout=3)
        except subprocess.TimeoutExpired:p.kill()
    server.shutdown()

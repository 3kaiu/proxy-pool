import subprocess
import os
import sys
import time

BINARY = os.path.join(os.path.dirname(os.path.abspath(__file__)), "relay-linux-amd64")
os.chmod(BINARY, 0o755)

env = os.environ.copy()
env["RELAY_LISTEN"] = "0.0.0.0:7860"
env["RELAY_TARGET"] = "https://opencode.ai/zen"
env.setdefault("RELAY_MAX_RETRIES", "50")
env.setdefault("RELAY_TIMEOUT", "5")

print(f"Starting relay binary: {BINARY}", flush=True)

proc = subprocess.Popen([BINARY], env=env, stdout=sys.stdout, stderr=sys.stderr)
print(f"Relay PID={proc.pid}, listening on :7860", flush=True)

while True:
    time.sleep(15)
    rc = proc.poll()
    if rc is not None:
        print(f"Relay exited (code={rc}), restarting...", flush=True)
        proc = subprocess.Popen([BINARY], env=env, stdout=sys.stdout, stderr=sys.stderr)
        print(f"Relay restarted PID={proc.pid}", flush=True)

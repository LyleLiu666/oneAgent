import pexpect
import sys
import re
import time

class Observer:
    """
    Mock Observer LLM.
    In production, this would call an actual LLM API with context.
    """
    def evaluate(self, tool_output, risk_level="unknown"):
        clean_text = re.sub(r'\x1b\[[0-9;]*m', '', tool_output)
        
        print(f"\n[Observer] Analyzing request...")
        # print(f"[Observer] Context: {clean_text.strip()[:200]}...")
        
        if "rm -rf" in clean_text:
            print("[Observer] 🛑 BLOCKED: High risk command detected.")
            return False
        
        print("[Observer] ✅ APPROVED: Safe operation.")
        return True

def run_safe_claude(initial_prompt):
    cmd = "claude" 
    print(f"[Wrapper] Starting '{cmd}'...")
    
    try:
        # Spawn the child process
        # Using a slightly larger dimensions to avoid line wrapping weirdness
        child = pexpect.spawn(cmd, encoding='utf-8', timeout=60, dimensions=(40, 120))
        child.logfile = sys.stdout
        
        # 1. Handle startup sequences
        # We assume we might see a Trust Prompt OR already be at logic
        
        while True:
            index = child.expect([
                r">",                                  # 0: Prompt ready
                r"Do you trust the files",             # 1: Trust dialog
                r"Visit .* to log in",                 # 2: Login
                r"Welcome to Claude Code",             # 3: Banner
                pexpect.EOF,
                pexpect.TIMEOUT
            ])
            
            if index == 0:
                print("\n[Wrapper] Prompt detected.")
                break # Ready to send command
                
            elif index == 1:
                print("\n[Wrapper] Trust prompt detected. Accepting...")
                child.sendline("\r") # Send Enter to accept default "Yes"
                # Loop back to wait for prompt
                continue
                
            elif index == 2:
                print("\n[Wrapper] 🚨 Auth Required! See URL above.")
                child.close()
                return
                
            elif index == 3:
                # specific banner, just wait
                print("\n[Wrapper] Welcome banner...")
                continue
                
            elif index == 4: # EOF
                print("\n[Wrapper] Unexpected EOF.")
                return
            elif index == 5: # TIMEOUT
                print("\n[Wrapper] Startup timeout.")
                return

        print("\n[Wrapper] Ready. Sending prompt...")
        child.sendline(initial_prompt)
        
        # 3. Main Event Loop
        observer = Observer()
        
        while True:
            # We look for prompts. 
            # Note: "Confirm?" prompts usually end the buffer.
            
            index = child.expect([
                r"\(y\/n\)",                   # 0
                r"\[y\/N\]",                   # 1
                r"> $",                       # 2: Main prompt returned
                pexpect.TIMEOUT,
                pexpect.EOF
            ])
            
            context = child.before
            
            if index == 0 or index == 1:
                print(f"\n[Wrapper] 🛡️  INTERCEPTION: Permission requested.")
                allowed = observer.evaluate(context)
                if allowed:
                    child.sendline("y")
                else:
                    child.sendline("n")
                    
            elif index == 2:
                # Main prompt. Check if we have output or if it's just the start.
                if len(context.strip()) > 3:
                    print("\n[Wrapper] Task seems complete (Prompt returned).")
                    break
                
            elif index == 3:
                # Timeout is not necessarily bad, might be streaming.
                # But pexpect expect loop waits for match. 
                # If streaming happens, it fills buffer but doesn't match keys.
                # We can check child.before to see progress.
                print(".", end="", flush=True)
                continue
                
            elif index == 4:
                print("\n[Wrapper] Exited.")
                break
        
        child.close()
        
    except Exception as e:
        print(f"\n[Wrapper] Error: {e}")

if __name__ == "__main__":
    prompt = "echo 'Hello Wrapper'"
    if len(sys.argv) > 1:
        prompt = " ".join(sys.argv[1:])
        
    run_safe_claude(prompt)

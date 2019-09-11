import subprocess
import Tkinter as tk
from tkMessageBox import *

p = subprocess.call\
    ('mono -v',shell=True, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE)


root = tk.Tk()


def on_closing():
    if showwarning("Quit", "Do you want to quit?"):
        root.destroy()


root.protocol("WM_DELETE_WINDOW", on_closing)
root.mainloop()



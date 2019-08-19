import tkinter as tk

root = tk.Tk()
root.attributes('-alpha', 0.0) #For icon
#root.lower()
root.iconify()
window = tk.Toplevel(root)
window.geometry("100x100") #Whatever size
window.overrideredirect(1) #Remove border
#window.attributes('-topmost', 1)
#Whatever buttons, etc 
close = tk.Button(window, text = "Close Window", command = lambda: root.destroy())
close.pack(fill = tk.BOTH, expand = 1)
window.mainloop()
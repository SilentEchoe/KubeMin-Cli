from io import StringIO
f = StringIO()
f.write('hello')
print(f.getvalue())









_path='C:\\Users\Lenovo\Desktop\密码.txt'
with open(_path, 'r') as f:
    f.read()
    for line in f.readlines():
        line.strip()




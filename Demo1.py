from io import StringIO
import os

f = StringIO()
f.write('hello')
print(f.getvalue())

# 操作系统 posix为MAC OS X 或 Linux ，Unix . Nt则为WINDOWS
os.name
# 系统环境变量
print(os.environ)

print(os.environ.get('PATH'))

print(os.environ.get('x', 'default'))

# 查看当前目录的绝对路径
_path = os.path.abspath('.')
# 在某个目录下创建一个新目录，首先把新目录的完整路径表示出来
_testpath = os.path.join('D:\Python_git', 'testdir')
# 创建一个文件夹
# os.mkdir(_testpath)
# 删除一个文件夹
# os.rmdir(_testpath)

# 把两个路径合成一个时，要通过 os.path.join() ， 可以正确处理不同操作系统的路径分割符
# 拆封路径的时候要使用 os.path.split()
print(os.path.split(_testpath))

os.path.splitext('/path/to/file.txt')
print(os.path.splitext('/path/to/file.txt'))


# shutil 为OS模块的补充













_path='C:\\Users\Lenovo\Desktop\密码.txt'
with open(_path, 'r') as f:
    f.read()
    for line in f.readlines():
        line.strip()




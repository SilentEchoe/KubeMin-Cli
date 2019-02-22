# -*- coding: utf-8 -*-
# 使用标准UTF-8编码

from multiprocessing import Process
from multiprocessing import Pool
import subprocess
import os, time, random


# print('Process (%s) start....' % os.getpid())
# windows 中没有fork()方法调用，只有在linux or mac.. 中才有
# pid = os.fork()
# if pid == 0:
#    print('I am child process (%s) and my parent is %s.' % (os.getpid(), os.getpid()))
# else:
#    print('I (%s) just created a child process (%s).' % (os.getpid(), pid))

# 子进程要执行的代码
def run_proc(name):
    print('Run child process %s (%s)...' % (name, os.getpid()))


if __name__ == '__main__':
    print('Parent process %s.' % os.getpid())
    p = Process(target=run_proc, args=('test',))
    print('Child process will start.')
    p.start()
    p.join()
    print('Child process end.')


# Pool
def long_time_task(name):
    print('Run task %s (%s)...' % (name, os.getpid()))
    start = time.time()
    time.sleep(random.random()*3)
    end = time.time()
    print('Task %s runs %0.2f seconds.' % (name, (end-start)))


if __name__=='__main__':
    print('Parent process %s.' % os.getpid())
    p = Pool(4)
    for i in  range(5):
        p.apply_async(long_time_task, args=(i,))
    print('Waiting for all subprocesses done...')
    p.close()
    p.join()
    print('All subprocesses done')


# print('$ nslookup www.python.org')
# r = subprocess.call(['nslookup', 'www.python.org'])
# print('Exit code:', r)

print('$ nslookup')
p = subprocess.Popen(['nslookup'], stdin=subprocess.PIPE, stdout=subprocess.PIPE,stderr=subprocess.PIPE)
output, err = p.communicate(b'set q=mx\npython.org\nexit\n')
print(output.decode('utf-8'))
print('Exit code:', p.returncode)
import websockets
import asyncio
import socket
import threading
import time

#async def hello(websocket, path):
#    name = await  websocket.recccv()
#    print(f"<{name}")

#     greeting = f"Hello{name}!"

#     await  websocket.send(greeting)
#     print(f">{greeting}")

# start_server = websockets.serve(hello, 'localhost', 8745)

# asyncio.get_event_loop().run_until_complete(start_server)
# asyncio.get_event_loop().run_forever()


s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
s.bind('127.0.0.1')

# 设置最大连接数量
s.listen(5)
print('Waiting for connection...')


def tcplink(sock, addr):
    print('Accept new connection from %s:%s...' % addr)
    sock.send(b'Welcome!')
    while True:
        data = sock.recv(1024)
        time.sleep(1)
        if not data or data.decode('utf-8') == 'exit':
            break
        sock.send(('Hello, %s!' % data.decode('utf-8')).encode('utf-8'))
    sock.close()
    print('Connection from %s:%s closed.' % addr)


while True:
    # 接受一个新连接:
    sock, addr = s.accept()
    # 创建新线程来处理TCP连接:
    t = threading.Thread(target=tcplink, args=(sock, addr))
    t.start()





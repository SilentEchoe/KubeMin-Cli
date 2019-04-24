import socket

# AF_INET指定使用IPV4
s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
s.connect(('www.sina.com.cn', 80))
# 发送数据



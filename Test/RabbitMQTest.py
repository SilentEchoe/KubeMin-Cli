import pika


credentials = pika.PlainCredentials('admin', 'admin')
connection = pika.BlockingConnection(pika.ConnectionParameters('127.0.0.1', 15672, '/', credentials))

print(connection)
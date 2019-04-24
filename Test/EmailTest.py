from email.mime.text import MIMEText
import smtplib

msg = MIMEText('Hello, Send by Pyhon...', 'plain', 'utf-8')
from_addr = input('From:')
password = input('Password:')
# 输入收件人地址
to_addr = input('To:')
# 输入SMYP 服务地址：
smtp_server = input('SMTP servier:')





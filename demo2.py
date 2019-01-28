#!/usr/bin/env python3
# 第一行注释可以直接在Unix/Linux/Mac
# -*- coding: utf-8 -*-
# 使用标准UTF-8编码

""" a test module """

import os
import sys
import json
import base64
import binascii

filePath = 'D:/Python_git/quotetutorial/user_ warehouse.json'
binPath = 'D:/Python_git/quotetutorial/bin1.bin'
file = open(filePath, 'r', encoding='utf-8')
jsonObj = json.load(file)
jsonObj = jsonObj.get("RECORDS")
count = 0

def decode(s):
    return ''.join([chr(i) for i in [int(b, 2) for b in s.split(' ')]])


def encode(s):
    return ' '.join([bin(ord(c)).replace('0b', '') for c in s])

while count < len(jsonObj):
    _fileName = jsonObj[count]['modal_name']
    _base64Obj = jsonObj[count]['bin_base64']
    # base64 转字符串
    _strbase64 = base64.b64decode(_base64Obj)
    print(_strbase64)
    _base64to2 = encode(_base64Obj)
    print(_base64to2)
    output_file = open(binPath, 'w')
    output_file.write(_base64to2)
    output_file.close()

    print(_base64Obj)
    count = count + 1

print('END')

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
while count < len(jsonObj):
    _fileName = jsonObj[count]['modal_name']
    _base64Obj = jsonObj[count]['bin_base64']
    input_file = open(binPath, 'w')
    h = binascii.b2a_hex(_base64Obj)

    decoded = base64.b64decode(_base64Obj)
    jm = base64.b64decode(_base64Obj)
    print(bin(int(jm, 16)))
    output_file = open(binPath, 'w')
    output_file.write(str(decoded))
    output_file.close()

    print(_base64Obj)
    count = count + 1

print('END')

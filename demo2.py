#!/usr/bin/env python3
# 第一行注释可以直接在Unix/Linux/Mac
# -*- coding: utf-8 -*-
# 使用标准UTF-8编码

""" a test module """

import json
import base64
import clr
import sys

sys.path.append("D:\myclass")
clr.AddReference('PythonNetTest')
from PythonNetTest import *
instance = BinFile()

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
    _bytebase64 = instance.Base64ToBytes(_base64Obj)
    instance.WriteByteToFile(_bytebase64, binPath)
    count = count + 1

print('END')

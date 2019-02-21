import pickle
import json
d = dict(name='Bob', age=20, score=88)
# pickle.dumps() 方法把任意对象序列化成 Bytes
print(pickle.dumps(d))
# pickle.dump() 把对象序列化后写入一个file-like Object
f = open('dump.txt', 'wb')
pickle.dump(d, f)
f.close()
# pickle.loads()方法反序列化出对象

json.dumps(d)


class Student(object):
    def __init__(self, name, age, score):
        self.name = name
        self.age = age
        self.score = score


s = Student('Box', 20, 88)


def student2dict(std):
    return {
        'name': std.name,
        'age': std.age,
        'score': std.score
        }


print(json.dumps(s, default=student2dict))
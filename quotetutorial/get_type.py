print(type('str'))
print(type(123))


class Student(object):
    pass


s = Student()
s.name = 'Michael'
print(s.name)


def set_age(self, age):
    self.age = age


from types import MethodType
s.set_age = MethodType(set_age, s)
s.set_age(25)
print(s.age)
from enum import Enum, unique


class Student(object):
    def __init__(self, name):
        self.name = name

    def __str__(self):
        return 'Student object(name: %s)' % self.name


print(Student('aa'))


Month = Enum('Month',('1','2','3'))
for name, member in Month.__members__.items():
    print(name, '=>', member, ',', member.value)

@unique
class Weekday(Enum):
    sun = 0
    Mon = 1
    Tue = 2


day1 = Weekday.Mon
print(Weekday.Tue.value)





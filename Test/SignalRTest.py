class Student(object):
    def __init__(self, name, age):
        self.name = name
        self.age = age

    def study(self, course_name):
        print('%s正在学习%s.' % (self.name,course_name))

    def watch_movie(self):
        if self.age < 18:
            print('%s只能看d电视.' % self.name)
        else:
            print('%s正在观看电影.' % self.name)

def main():
    bart = Student('王凯', 38)
    bart.study('Python程序设计')
    bart.watch_movie()







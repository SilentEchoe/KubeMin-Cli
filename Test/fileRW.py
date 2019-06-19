def main():
    path = 'C://a.txt'
    f = open(path, 'r', encoding='utf-8')
    print(f.read())
    f.close()


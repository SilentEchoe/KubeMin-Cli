import xlrd


file = 'C:\\Users\Lenovo\Desktop\脚本\Ge.xls'


def read_excel():
    # 打开文件
    wb = xlrd.open_workbook(filename=file)

    # 通过索引获取表格
    sheet1 = wb.sheet_by_index(0)

    print(sheet1.nrows)

    # 获取行内容
    rows = sheet1.row_values(2)

    # 获取列内容
    cols = sheet1.col_values(2)

    print(rows[0])
    print(rows)


if __name__ == '__main__':
    read_excel()


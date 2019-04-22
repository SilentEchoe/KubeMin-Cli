import json
from openpyxl import Workbook

_jsonFilePath = 'D:/response_1555912727699.json'
with open(_jsonFilePath, 'r') as f:
    data = json.loads(f.read())
    result = [(item.get('id', 'NA'), item.get('modalName', 'NA'), item.get('modalName', 'NA'), item.get('attrKey', 'NA'), item.get('attrVlue', 'NA'), item.get('compatibleType', 'NA'), item.get('isExist', 'NA')) for item in data['obj']]


wb = Workbook()
sheet = wb.active
sheet.title = "New Shit"

i = 0
_id = 0
for iterating_var in result:
    if (iterating_var[0] ==_id):
        sheet["I%d" % (i)].value = iterating_var[3]
        sheet["J%d" % (i)].value = iterating_var[4]
        sheet["K%d" % (i)].value = iterating_var[5]
        sheet["L%d" % (i)].value = iterating_var[6]

    else:
        sheet["A%d" % (i + 1)].value = iterating_var[0]
        sheet["B%d" % (i + 1)].value = iterating_var[1]
        sheet["C%d" % (i + 1)].value = iterating_var[2]
        sheet["D%d" % (i + 1)].value = iterating_var[3]
        sheet["E%d" % (i + 1)].value = iterating_var[4]
        sheet["F%d" % (i + 1)].value = iterating_var[5]
        sheet["G%d" % (i + 1)].value = iterating_var[6]
    i = i+1
    _id = iterating_var[0]
    print(_id)




wb.save('保存一个新的excel.xlsx')


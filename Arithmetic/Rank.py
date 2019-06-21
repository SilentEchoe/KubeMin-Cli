def select_sort(origin_items, comp=lambda x, y: x < y):
    """简单排序"""
    items = origin_items[:]
    for i in range(len(items)-1):
        min_index = i
        for j in range(i + 1, len(items)):
            if comp(items[j], items[min_index]):
                min_index = j
            items[i], items[min_index] = items[min_index], items[i]
    return items
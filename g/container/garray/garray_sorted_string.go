// Copyright 2018 gf Author(https://gitee.com/johng/gf). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://gitee.com/johng/gf.

package garray

import (
    "gitee.com/johng/gf/g/container/gtype"
    "gitee.com/johng/gf/g/internal/rwmutex"
    "gitee.com/johng/gf/g/util/gconv"
    "gitee.com/johng/gf/g/util/grand"
    "math"
    "sort"
    "strings"
)

// 默认按照从小到大进行排序
type SortedStringArray struct {
    mu          *rwmutex.RWMutex        // 互斥锁
    array       []string                // 底层数组
    unique      *gtype.Bool             // 是否要求不能重复
    compareFunc func(v1, v2 string) int // 比较函数，返回值 -1: v1 < v2；0: v1 == v2；1: v1 > v2
}

// Create an empty sorted array.
// The param <unsafe> used to specify whether using array with un-concurrent-safety,
// which is false in default, means concurrent-safe in default.
//
// 创建一个空的排序数组对象，参数unsafe用于指定是否用于非并发安全场景，默认为false，表示并发安全。
func NewSortedStringArray(unsafe...bool) *SortedStringArray {
    return NewSortedStringArraySize(0, unsafe...)
}

func NewSortedStringArraySize(cap int, unsafe...bool) *SortedStringArray {
    return &SortedStringArray {
        mu          : rwmutex.New(unsafe...),
        array       : make([]string, 0, cap),
        unique      : gtype.NewBool(),
        compareFunc : func(v1, v2 string) int {
            return strings.Compare(v1, v2)
        },
    }
}

func NewSortedStringArrayFrom(array []string, unsafe...bool) *SortedStringArray {
    a := NewSortedStringArraySize(0, unsafe...)
    a.array = array
    sort.Strings(a.array)
    return a
}

// 设置底层数组变量.
func (a *SortedStringArray) SetArray(array []string) *SortedStringArray {
    a.mu.Lock()
    defer a.mu.Unlock()
    a.array = array
    sort.Strings(a.array)
    return a
}

// 将数组重新排序(从小到大).
func (a *SortedStringArray) Sort() *SortedStringArray {
    a.mu.Lock()
    defer a.mu.Unlock()
    sort.Strings(a.array)
    return a
}

// 添加加数据项
func (a *SortedStringArray) Add(values...string) *SortedStringArray {
    if len(values) == 0 {
        return a
    }
    a.mu.Lock()
    defer a.mu.Unlock()
    for _, value := range values {
        index, cmp := a.binSearch(value, false)
        if a.unique.Val() && cmp == 0 {
            continue
        }
        if index < 0 {
            a.array = append(a.array, value)
            continue
        }
        // 加到指定索引后面
        if cmp > 0 {
            index++
        }
        rear   := append([]string{}, a.array[index : ]...)
        a.array = append(a.array[0 : index], value)
        a.array = append(a.array, rear...)
    }
    return a
}

// 获取指定索引的数据项, 调用方注意判断数组边界
func (a *SortedStringArray) Get(index int) string {
    a.mu.RLock()
    defer a.mu.RUnlock()
    value := a.array[index]
    return value
}

// 删除指定索引的数据项, 调用方注意判断数组边界
func (a *SortedStringArray) Remove(index int) string {
    a.mu.Lock()
    defer a.mu.Unlock()
    // 边界删除判断，以提高删除效率
    if index == 0 {
        value  := a.array[0]
        a.array = a.array[1 : ]
        return value
    } else if index == len(a.array) - 1 {
        value  := a.array[index]
        a.array = a.array[: index]
        return value
    }
    // 如果非边界删除，会涉及到数组创建，那么删除的效率差一些
    value  := a.array[index]
    a.array = append(a.array[ : index], a.array[index + 1 : ]...)
    return value
}

// 将最左端(索引为0)的数据项移出数组，并返回该数据项
func (a *SortedStringArray) PopLeft() string {
    a.mu.Lock()
    defer a.mu.Unlock()
    value  := a.array[0]
    a.array = a.array[1 : ]
    return value
}

// 将最右端(索引为length - 1)的数据项移出数组，并返回该数据项
func (a *SortedStringArray) PopRight() string {
    a.mu.Lock()
    defer a.mu.Unlock()
    index  := len(a.array) - 1
    value  := a.array[index]
    a.array = a.array[: index]
    return value
}

// Calculate the sum of values in an array.
//
// 对数组中的元素项求和(将元素值转换为int类型后叠加)。
func (a *SortedStringArray) Sum() (sum int) {
    a.mu.RLock()
    defer a.mu.RUnlock()
    for _, v := range a.array {
        sum += gconv.Int(v)
    }
    return
}

// 数组长度
func (a *SortedStringArray) Len() int {
    a.mu.RLock()
    length := len(a.array)
    a.mu.RUnlock()
    return length
}

// 返回原始数据数组
func (a *SortedStringArray) Slice() []string {
    array := ([]string)(nil)
    if a.mu.IsSafe() {
        a.mu.RLock()
        defer a.mu.RUnlock()
        array = make([]string, len(a.array))
        copy(array, a.array)
    } else {
        array = a.array
    }
    return array
}

// 查找指定数值是否存在
func (a *SortedStringArray) Contains(value string) bool {
    _, r := a.Search(value)
    return r == 0
}

// 查找指定数值的索引位置，返回索引位置(具体匹配位置或者最后对比位置)及查找结果
// 返回值: 最后比较位置, 比较结果
func (a *SortedStringArray) Search(value string) (index int, result int) {
    return a.binSearch(value, true)
}

func (a *SortedStringArray) binSearch(value string, lock bool) (index int, result int) {
    if len(a.array) == 0 {
        return -1, -2
    }
    if lock {
        a.mu.RLock()
        defer a.mu.RUnlock()
    }
    min := 0
    max := len(a.array) - 1
    mid := 0
    cmp := -2
    for min <= max {
        mid = int((min + max) / 2)
        cmp = a.compareFunc(value, a.array[mid])
        switch {
            case cmp < 0 : max = mid - 1
            case cmp > 0 : min = mid + 1
            default :
                return mid, cmp
        }
    }
    return mid, cmp
}

// 设置是否允许数组唯一
func (a *SortedStringArray) SetUnique(unique bool) *SortedStringArray {
    oldUnique := a.unique.Val()
    a.unique.Set(unique)
    if unique && oldUnique != unique {
        a.Unique()
    }
    return a
}

// 清理数组中重复的元素项
func (a *SortedStringArray) Unique() *SortedStringArray {
    a.mu.Lock()
    i := 0
    for {
        if i == len(a.array) - 1 {
            break
        }
        if a.compareFunc(a.array[i], a.array[i + 1]) == 0 {
            a.array = append(a.array[ : i + 1], a.array[i + 1 + 1 : ]...)
        } else {
            i++
        }
    }
    a.mu.Unlock()
    return a
}

// Return a new array, which is a copy of current array.
//
// 克隆当前数组，返回当前数组的一个拷贝。
func (a *SortedStringArray) Clone() (newArray *SortedStringArray) {
    a.mu.RLock()
    array := make([]string, len(a.array))
    copy(array, a.array)
    a.mu.RUnlock()
    return NewSortedStringArrayFrom(array, !a.mu.IsSafe())
}

// 清空数据数组
func (a *SortedStringArray) Clear() *SortedStringArray {
    a.mu.Lock()
    if len(a.array) > 0 {
        a.array = make([]string, 0)
    }
    a.mu.Unlock()
    return a
}

// 使用自定义方法执行加锁修改操作
func (a *SortedStringArray) LockFunc(f func(array []string)) *SortedStringArray {
    a.mu.Lock(true)
    defer a.mu.Unlock(true)
    f(a.array)
    return a
}

// 使用自定义方法执行加锁读取操作
func (a *SortedStringArray) RLockFunc(f func(array []string)) *SortedStringArray {
    a.mu.RLock(true)
    defer a.mu.RUnlock(true)
    f(a.array)
    return a
}

// 合并两个数组.
func (a *SortedStringArray) Merge(array *SortedStringArray) *SortedStringArray {
    a.mu.Lock()
    defer a.mu.Unlock()
    if a != array {
        array.mu.RLock()
        defer array.mu.RUnlock()
    }
    a.array = append(a.array, array.array...)
    sort.Strings(a.array)
    return a
}

// Chunks an array into arrays with size elements. The last chunk may contain less than size elements.
//
// 将一个数组分割成多个数组，其中每个数组的单元数目由size决定。最后一个数组的单元数目可能会少于size个。
func (a *SortedStringArray) Chunk(size int) [][]string {
    if size < 1 {
        panic("size: cannot be less than 1")
    }
    a.mu.RLock()
    defer a.mu.RUnlock()
    length := len(a.array)
    chunks := int(math.Ceil(float64(length) / float64(size)))
    var n [][]string
    for i, end := 0, 0; chunks > 0; chunks-- {
        end = (i + 1) * size
        if end > length {
            end = length
        }
        n = append(n, a.array[i*size : end])
        i++
    }
    return n
}

// Extract a slice of the array(If in concurrent safe usage, it returns a copy of the slice; else a pointer).
// It returns the sequence of elements from the array array as specified by the offset and length parameters.
//
// 返回根据offset和size参数所指定的数组中的一段序列。
func (a *SortedStringArray) SubSlice(offset, size int) []string {
    a.mu.RLock()
    defer a.mu.RUnlock()
    if offset > len(a.array) {
        return nil
    }
    if offset + size > len(a.array) {
        size = len(a.array) - offset
    }
    if a.mu.IsSafe() {
        s := make([]string, size)
        copy(s, a.array[offset:])
        return s
    } else {
        return a.array[offset:]
    }
}

// Picks one or more random entries out of an array(a copy), and returns the key (or keys) of the random entries.
//
// 从数组中随机取出size个元素项，构成slice返回。
func (a *SortedStringArray) Rand(size int) []string {
    a.mu.RLock()
    defer a.mu.RUnlock()
    if size > len(a.array) {
        size = len(a.array)
    }
    n := make([]string, size)
    for i, v := range grand.Perm(len(a.array)) {
        n[i] = a.array[v]
        if i == size - 1 {
            break
        }
    }
    return n
}

// Join array elements with a string.
//
// 使用glue字符串串连当前数组的元素项，构造成新的字符串返回。
func (a *SortedStringArray) Join(glue string) string {
    a.mu.RLock()
    defer a.mu.RUnlock()
    return strings.Join(a.array, glue)
}
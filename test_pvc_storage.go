package main

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// 复制修复后的PVC存储比较函数
func compareStorageSmart(a, b corev1.ResourceList) bool {
	// 对于PVC存储，零值应该视为使用默认值，而不是"未设置"
	aStorage := getPVCStorageValue(a)
	bStorage := getPVCStorageValue(b)
	return aStorage == bStorage
}

// getPVCStorageValue 获取PVC存储值，零值使用默认值
func getPVCStorageValue(resources corev1.ResourceList) string {
	if val, ok := resources[corev1.ResourceStorage]; ok {
		// 如果值是零，返回默认PVC大小
		if val.IsZero() {
			return "1Gi" // PVC默认大小
		}
		return val.String()
	}
	return "1Gi" // 资源未设置，使用默认大小
}

// 复制Deployment资源比较函数用于对比
func compareResourceListSmart(a, b corev1.ResourceList) bool {
	// 比较 CPU 资源
	aCPU := getResourceValueSmart(a, corev1.ResourceCPU)
	bCPU := getResourceValueSmart(b, corev1.ResourceCPU)
	cpuEqual := aCPU == bCPU

	// 比较内存资源
	aMem := getResourceValueSmart(a, corev1.ResourceMemory)
	bMem := getResourceValueSmart(b, corev1.ResourceMemory)
	memEqual := aMem == bMem

	return cpuEqual && memEqual
}

// getResourceValueSmart 智能获取资源值，将零值视为未设置（用于CPU/内存）
func getResourceValueSmart(resources corev1.ResourceList, name corev1.ResourceName) string {
	if val, ok := resources[name]; ok {
		// 如果值是零，返回空字符串表示"未设置"
		if val.IsZero() {
			return ""
		}
		return val.String()
	}
	return "" // 资源未设置
}

func main() {
	fmt.Println("🧪 Testing PVC storage vs CPU/memory resource comparison...")

	// 场景1：PVC存储比较 - 零值应该使用默认值
	fmt.Println("\n💾 PVC Storage Comparison:")
	storage1 := corev1.ResourceList{
		corev1.ResourceStorage: resource.MustParse("10Gi"),
	}
	storage2 := corev1.ResourceList{
		corev1.ResourceStorage: resource.MustParse("10Gi"),
	}
	result1 := compareStorageSmart(storage1, storage2)
	fmt.Printf("   10Gi vs 10Gi: %t (expected: true)\n", result1)

	// 零值存储 - 应该使用默认值1Gi
	zeroStorage := corev1.ResourceList{
		corev1.ResourceStorage: resource.MustParse("0"),
	}
	fmt.Printf("   10Gi storage: %v (IsZero: %t) -\u003e Smart value: '%s'\n",
		storage1.Storage(), storage1.Storage().IsZero(), getPVCStorageValue(storage1))
	fmt.Printf("   Zero storage: %v (IsZero: %t) -\u003e Smart value: '%s'\n",
		zeroStorage.Storage(), zeroStorage.Storage().IsZero(), getPVCStorageValue(zeroStorage))

	result2 := compareStorageSmart(storage1, zeroStorage)
	fmt.Printf("   10Gi vs 0Gi: %t (expected: false - 10Gi != 1Gi默认)\n", result2)

	// 空存储 vs 零值存储 - 都应该使用默认值1Gi
	emptyStorage := corev1.ResourceList{}
	result3 := compareStorageSmart(emptyStorage, zeroStorage)
	fmt.Printf("   Empty vs 0Gi: %t (expected: true - 都使用1Gi默认)\n", result3)

	// 场景2：CPU/内存比较 - 零值应该视为未设置
	fmt.Println("\n⚙️ CPU/Memory Comparison:")
	requests1 := corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse("100m"),
		corev1.ResourceMemory: resource.MustParse("128Mi"),
	}
	requests2 := corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse("100m"),
		corev1.ResourceMemory: resource.MustParse("128Mi"),
	}
	result4 := compareResourceListSmart(requests1, requests2)
	fmt.Printf("   100m/128Mi vs 100m/128Mi: %t (expected: true)\n", result4)

	// 零值CPU/内存 - 应该视为未设置
	zeroRequests := corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse("0"),
		corev1.ResourceMemory: resource.MustParse("0"),
	}
	fmt.Printf("   100m/128Mi requests: CPU='%s' Mem='%s'\n",
		getResourceValueSmart(requests1, corev1.ResourceCPU),
		getResourceValueSmart(requests1, corev1.ResourceMemory))
	fmt.Printf("   Zero requests: CPU='%s' Mem='%s'\n",
		getResourceValueSmart(zeroRequests, corev1.ResourceCPU),
		getResourceValueSmart(zeroRequests, corev1.ResourceMemory))

	result5 := compareResourceListSmart(requests1, zeroRequests)
	fmt.Printf("   100m/128Mi vs 0/0: %t (expected: true - 零值视为未设置)\n", result5)

	// 空请求 vs 零值请求 - 都应该视为未设置
	emptyRequests := corev1.ResourceList{}
	result6 := compareResourceListSmart(emptyRequests, zeroRequests)
	fmt.Printf("   Empty vs 0/0: %t (expected: true - 都视为未设置)\n", result6)

	fmt.Println("\n📋 Summary:")
	fmt.Println("✅ PVC存储：零值使用默认值（1Gi），确保PVC可用性")
	fmt.Println("✅ CPU/内存：零值视为未设置，避免不必要的Deployment更新")
	fmt.Println("✅ 这种差异化处理确保了不同类型资源的正确行为")

	fmt.Println("\n🔍 Key Differences:")
	fmt.Println("   - PVC存储：0Gi → 1Gi（默认，确保可用性）")
	fmt.Println("   - CPU/内存：0 → ''（视为未设置，避免误更新）")
	fmt.Println("   - 实际值：保持原值（正确识别变化）")
}

func int32Ptr(i int32) *int32 {
	return &i
}
package main

import (
	"fmt"
	firstpb "github.com/AnAnonymousFriend/LearningNotes-Go/src/first"
	enumpb "github.com/AnAnonymousFriend/LearningNotes-Go/src/second"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"io/ioutil"
	"log"
)

func main()  {
	//pm := NewPersonMessage()
	//writeToFile("person.bin",pm)

	pm2 := &firstpb.PersonMessage{}
	_ = readFromFile("person.bin", pm2)
	fmt.Println(pm2)

	pmString := toJSON(pm2)
	fmt.Println(pmString)

	em := NewEnumMessage()

	fmt.Println(enumpb.Gender_name[int32(em.Gender)])

}

func readFromFile(fileName string, pb proto.Message) error  {
	dataBytes, err := ioutil.ReadFile(fileName)
	if err != nil {
		log.Fatalln("读取文件时发生错误",err.Error())
	}
	err = proto.Unmarshal(dataBytes,pb)
	if err !=nil {
		log.Fatalln("转换为结构体的时候发生错误",err.Error())
	}

	return nil

}

func NewEnumMessage() *enumpb.EnumMessage{
	em := enumpb.EnumMessage{
		Id: 345,
		Gender : enumpb.Gender_FEMALE,
	}
	em.Gender = enumpb.Gender_FEMALE

	return  &em
}



func toJSON(pb proto.Message)string  {
 	marshaler := jsonpb.Marshaler{Indent:"     "}
 	
 	str ,err := marshaler.MarshalToString(pb)
	if err !=nil {
		log.Fatalln("转换为Json时发生错误", err.Error())
	}
	
	return  str
}

func writeToFile(fileName string, pb proto.Message) error  {
	// 序列化
	dataBytes, err := proto.Marshal(pb)
	if err != nil {
		log.Fatalln("无法序列化")
	}

	if err := ioutil.WriteFile(fileName,dataBytes, 0644);
	err != nil {
		log.Fatalln("无法写入到文件", err.Error())
	}
log.Println("成功")

return nil
}


func NewPersonMessage() *firstpb.PersonMessage {
	pm := firstpb.PersonMessage{
		Id:          1234,
		IsAdult:     true,
		Name:        "Dave",
		LuckNumvers: []int32{1,2,3,4,5},

	}

	fmt.Println(pm)

	pm.Name="Nick"

	fmt.Println(pm)

	fmt.Printf("The Id is %d\n",pm.GetId())

	return  &pm
}




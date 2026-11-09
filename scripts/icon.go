//go:build ignore

package main

import (
 "bytes"
 "encoding/binary"
 "image"
 "image/color"
 "image/png"
 "os"
)
func main(){img:=image.NewNRGBA(image.Rect(0,0,128,128));teal:=color.NRGBA{35,104,92,255};paper:=color.NRGBA{245,247,237,255};for y:=8;y<120;y++{for x:=8;x<120;x++{img.Set(x,y,teal)}};for y:=36;y<94;y++{for x:=30;x<98;x++{if x<60||x>67{img.Set(x,y,paper)}}};for y:=43;y<86;y++{for x:=36;x<92;x++{if x<55||x>72{img.Set(x,y,teal)}}};var b bytes.Buffer;if err:=png.Encode(&b,img);err!=nil{panic(err)};if err:=os.MkdirAll("build",0755);err!=nil{panic(err)};if err:=os.WriteFile("build/appicon.png",b.Bytes(),0644);err!=nil{panic(err)};var ico bytes.Buffer;for _,n:=range []uint16{0,1,1}{_ = binary.Write(&ico,binary.LittleEndian,n)};ico.Write([]byte{128,128,0,0});for _,n:=range []uint16{1,32}{_ = binary.Write(&ico,binary.LittleEndian,n)};_ = binary.Write(&ico,binary.LittleEndian,uint32(b.Len()));_ = binary.Write(&ico,binary.LittleEndian,uint32(22));ico.Write(b.Bytes());if err:=os.MkdirAll("internal/native",0755);err!=nil{panic(err)};if err:=os.WriteFile("internal/native/icon.ico",ico.Bytes(),0644);err!=nil{panic(err)}}

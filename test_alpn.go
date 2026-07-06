//go:build ignore

package main; import (fmt; unsafe); type SecApplicationProtocolList struct { ProtoNegoExt uint32; ProtocolListSize uint16; ProtocolList [12]byte }; func main() { fmt.Println(unsafe.Sizeof(SecApplicationProtocolList{})) }

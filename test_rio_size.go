package main

/*
#include <windows.h>
#include <winsock2.h>
#include <mswsock.h>
*/
import "C"
import "fmt"
import "unsafe"

func main() {
	var n C.RIO_NOTIFICATION_COMPLETION
	fmt.Printf("Size: %d\n", unsafe.Sizeof(n))
}

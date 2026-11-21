package main

import (
	"fmt"

	"github.com/Kha-Beleh/KFM/internal/cli"
)

func main() {
	fmt.Println("Добро пожаловать в KFM!")
	fmt.Println("напечатайте help для вывода всех доступных команд")

	err := cli.Run()

	if err != nil {
		fmt.Println("ошибка: ", err)
		return
	}

	fmt.Println("До свидания!")
}

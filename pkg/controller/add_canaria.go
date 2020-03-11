package controller

import (
	"github.com/iyacontrol/canaria-controller/pkg/controller/canaria"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, canaria.Add)
}

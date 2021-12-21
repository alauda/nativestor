package controller

import (
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

func predicateController() predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			logger.Info("topolvm cluster create event")
			return true
		},

		UpdateFunc: func(e event.UpdateEvent) bool {
			logger.Info("topolvm cluster update event")
			return true
		},

		DeleteFunc: func(e event.DeleteEvent) bool {
			logger.Info("topolvm cluster delete event")
			return true
		},

		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}

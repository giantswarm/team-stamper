package controller

import (
    "reflect"

    v1 "k8s.io/api/core/v1"
    "sigs.k8s.io/controller-runtime/pkg/event"
    "sigs.k8s.io/controller-runtime/pkg/predicate"
)

var ConfigMapDataChangedPredicate = predicate.Funcs{
    CreateFunc: func(e event.CreateEvent) bool {
        return true
    },
    DeleteFunc: func(e event.DeleteEvent) bool {
        return false
    },
    UpdateFunc: func(e event.UpdateEvent) bool {
        oldCm, ok := e.ObjectOld.(*v1.ConfigMap)

        if !ok {
            return false
        }

        newCm, ok := e.ObjectNew.(*v1.ConfigMap)
        if !ok {
            return false
        }

        return !reflect.DeepEqual(oldCm.Data, newCm.Data)
    },
    GenericFunc: func(e event.GenericEvent) bool {
        return false
    },
}

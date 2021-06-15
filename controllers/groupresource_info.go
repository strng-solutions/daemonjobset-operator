package controllers

import "k8s.io/apimachinery/pkg/runtime/schema"

var (
	GroupResource = schema.GroupResource{Group: "batch.strng.solutions", Resource: "DaemonJobSet"}
)

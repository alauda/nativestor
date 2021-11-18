package controllers

import (
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"
)

var (
	// ImmediateRetryResult Return this for a immediate retry of the reconciliation loop with the same request object.
	ImmediateRetryResult = reconcile.Result{Requeue: true}

	// ImmediateRetryResultNoBackoff Return this for a immediate retry of the reconciliation loop with the same request object.
	// Override the exponential backoff behavior by setting the RequeueAfter time explicitly.
	ImmediateRetryResultNoBackoff = reconcile.Result{Requeue: true, RequeueAfter: time.Second}

	// WaitForRequeueIfCephClusterNotReady waits for the CephCluster to be ready
	WaitForRequeueIfCephClusterNotReady = reconcile.Result{Requeue: true, RequeueAfter: 10 * time.Second}

	// WaitForRequeueIfFinalizerBlocked waits for resources to be cleaned up before the finalizer can be removed
	WaitForRequeueIfFinalizerBlocked = reconcile.Result{Requeue: true, RequeueAfter: 10 * time.Second}

	// WaitForRequeueIfOperatorNotInitialized waits for resources to be cleaned up before the finalizer can be removed
	WaitForRequeueIfOperatorNotInitialized = reconcile.Result{Requeue: true, RequeueAfter: 10 * time.Second}

	// OperatorCephBaseImageVersion is the ceph version in the operator image
	OperatorCephBaseImageVersion string
)

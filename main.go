package main

import (
	"github.com/appscode/jsonpatch"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/component-base/logs"
	"k8s.io/klog"

	"context"
	"encoding/json"
	"fmt"
	"time"
)

var clientset *kubernetes.Clientset

const addTruePatch = "[{\"op\": \"add\", \"path\": \"/status/conditions/-\", \"value\": {\"type\": \"www.example.com/feature-1\", \"status\": \"True\", \"lastProbeTime\": null}}]"
const addFalsePatch = "[{\"op\": \"add\", \"path\": \"/status/conditions/-\", \"value\": {\"type\": \"www.example.com/feature-1\", \"status\": \"False\", \"lastProbeTime\": null}}]"

func main() {
	logs.InitLogs()

	klog.Info("begin run")
	config, err := clientcmd.BuildConfigFromFlags("", "/Users/dutianpeng/.kube/config")
	if err != nil {
		klog.Info("get config error:", err)
		return
	}

	clientset, err = kubernetes.NewForConfig(config)
	podLW := cache.NewListWatchFromClient(
		clientset.CoreV1().RESTClient(),
		"pods",
		v1.NamespaceDefault,
		fields.Everything(),
	)

	_, podinformer := cache.NewInformer(
		podLW,
		&v1.Pod{},
		time.Second*30,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    handlepodsAdd,
			UpdateFunc: handlepodsupdate,
		},
	)

	var stopCh <-chan struct{}
	go podinformer.Run(nil)
	if !cache.WaitForCacheSync(stopCh, podinformer.HasSynced) {
		klog.Info("not sync")
		return
	}

	klog.Info("wait for stop")
	<-stopCh
}

func handlepodsupdate(_ interface{}, newObj interface{}) {
	handlepodsAdd(newObj)
}

func handlepodsAdd(obj interface{}) {
	pod := obj.(*v1.Pod)

	if pod.DeletionTimestamp != nil {
		return
	}
	//判断有无标签
	if v, ok := pod.Labels["example"]; ok {
		if v == "true" {
			have, status := getReadnessGateConditions(pod)
			if have == true && status == v1.ConditionTrue {
				return
			} else if have == false {
				klog.Info("如果需要设置为true,且现在没有，设置标签为true")
				_, err := clientset.
					CoreV1().
					Pods(pod.Namespace).
					Patch(context.TODO(),
						pod.Name,
						types.JSONPatchType,
						[]byte(addTruePatch),
						metav1.PatchOptions{},
						"status")
				if err != nil {
					klog.Info("patch pod error:", err)
					return
				}
			} else {
				klog.Info("如果需要设置为true,现在有，但是为false改为true")
				patch, err := getchangePatch(pod, true)
				if err != nil {
					klog.Info("get patch error:", err)
				}
				patchbytes, err := json.Marshal(patch)
				if err != nil {
					klog.Info("Marshal patch error:", err)
				}
				klog.Info("patch pod: ", string(patchbytes))
				_, err = clientset.
					CoreV1().
					Pods(pod.Namespace).
					Patch(context.TODO(),
						pod.Name,
						types.JSONPatchType,
						patchbytes,
						metav1.PatchOptions{},
						"status")
				if err != nil {
					klog.Info("Patch pod error:", err)
					return
				}
			}
		} else {
			have, status := getReadnessGateConditions(pod)
			if have == true && status == v1.ConditionFalse {
				return
			} else if have == false {
				klog.Info("如果需要设置为false,现在没有，则添加")
				_, err := clientset.
					CoreV1().
					Pods(pod.Namespace).
					Patch(context.TODO(),
						pod.Name,
						types.JSONPatchType,
						[]byte(addFalsePatch),
						metav1.PatchOptions{},
						"status")
				if err != nil {
					klog.Info(err)
					return
				}
			} else {
				klog.Info("如果需要设置为false,现在有，但是为true,则改为false")
				patch, err := getchangePatch(pod, false)
				if err != nil {
					klog.Info(err)
					return
				}
				patchbytes, err := json.Marshal(patch)
				if err != nil {
					klog.Info("Marshal patch error: ", err)
				}
				klog.Info("path pod:", string(patchbytes))
				_, err = clientset.
					CoreV1().
					Pods(pod.Namespace).
					Patch(context.TODO(),
						pod.Name,
						types.JSONPatchType,
						patchbytes,
						metav1.PatchOptions{},
						"status")
				if err != nil {
					klog.Info("Patch pod error", err)
					return

				}
			}
		}
	}
}

func getReadnessGateConditions(pod *v1.Pod) (bool, v1.ConditionStatus) {
	for _, v := range pod.Status.Conditions {
		if fmt.Sprint(v.Type) == "www.example.com/feature-1" {
			return true, v.Status
		}
	}
	return false, v1.ConditionUnknown
}

func getchangePatch(pod *v1.Pod, status bool) ([]jsonpatch.Operation, error) {
	pod1 := pod.DeepCopy()
	for k, v := range pod1.Status.Conditions {
		if v.Type == "www.example.com/feature-1" {
			if status {
				pod1.Status.Conditions[k].Status = v1.ConditionTrue
			} else {
				pod1.Status.Conditions[k].Status = v1.ConditionFalse
			}
		}
	}
	podjson, err := json.Marshal(pod)
	if err != nil {
		klog.Info("marshal origin pod error: ", err)
	}

	pod1json, err := json.Marshal(pod1)
	if err != nil {
		klog.Info("marshal new pod error: ", err)
	}
	return jsonpatch.CreatePatch(podjson, pod1json)
}
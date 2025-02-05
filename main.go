package main

import (
	"errors"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	klog "k8s.io/klog"
)

// Set on build
var version string

func main() {
	log.SetOutput(os.Stdout)
	log.SetFormatter(&log.JSONFormatter{})

	// Ensure klog also outputs to logrus
	klog.SetOutput(log.StandardLogger().WriterLevel(log.WarnLevel))

	app := cli.NewApp()
	app.Name = "Shawarma"
	app.Usage = "Sidecar for monitoring a Kubernetes service and notifying the main application when it is live"
	app.Copyright = "(c) 2019 CenterEdge Software"
	app.Version = version

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "log-level, l",
			Usage:  "Set the log level (panic, fatal, error, warn, info, debug, trace)",
			Value:  "warn",
			EnvVar: "LOG_LEVEL",
		},
		cli.StringFlag{
			Name:  "kubeconfig",
			Usage: "Path to a kubeconfig file, if not running in-cluster",
		},
	}
	app.Before = func(c *cli.Context) error {
		// In case of empty environment variable, pull default here too
		levelString := c.String("log-level")
		if levelString == "" {
			levelString = "warn"
		}

		level, err := log.ParseLevel(levelString)
		if err != nil {
			return err
		}

		log.SetLevel(level)

		return nil
	}

	app.Commands = []cli.Command{
		{
			Name:    "monitor",
			Aliases: []string{"m"},
			Usage:   "Monitor a Kubernetes service",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:   "label, l",
					Usage:  "Will monitor endpoints/services that have this label value",
					EnvVar: "ENDPOINT_LABEL",
				},
				cli.StringFlag{
					Name:   "pod, p",
					Usage:  "Kubernetes pod to monitor",
					EnvVar: "POD_NAME",
				},
				cli.StringFlag{
					Name:   "namespace, n",
					Value:  "default",
					Usage:  "Kubernetes namespace to monitor",
					EnvVar: "NAMESPACE",
				},
				cli.StringFlag{
					Name:   "url, u",
					Value:  "http://localhost/applicationstate",
					Usage:  "URL which receives a POST on state change",
					EnvVar: "SHAWARMA_URL",
				},
			},
			Action: func(c *cli.Context) error {
				name := c.String("label")
				if name == "" {
					return errors.New("label is required but was empty")
				}
				pod := c.String("pod")
				if pod == "" {
					return errors.New("pod is required but was empty")
				}
				info := monitorInfo{
					Namespace:    c.String("namespace"),
					PodName:      pod,
					Name:         name,
					URL:          c.String("url"),
					PathToConfig: c.GlobalString("kubeconfig"),
				}

				// In case of empty environment variable, pull default here too
				if info.URL == "" {
					info.URL = "http://localhost/applicationstate"
				}

				return monitorService(&info)
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

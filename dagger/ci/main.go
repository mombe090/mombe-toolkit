package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"dagger.io/dagger"
)

func main() {

	// check for Docker Hub registry credentials in host environment
	vars := []string{"DOCKERHUB_USERNAME", "DOCKERHUB_PASSWORD"}
	for _, v := range vars {
		if os.Getenv(v) == "" {
			log.Fatalf("Environment variable %s is not set", v)
		}
	}

	// initialize Dagger client
	ctx := context.Background()
	client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stderr))
	if err != nil {
		panic(err)
	}
	defer client.Close()

	// set registry password as secret for Dagger pipeline
	password := client.SetSecret("password", os.Getenv("DOCKERHUB_PASSWORD"))
	username := os.Getenv("DOCKERHUB_USERNAME")

	// create a cache volume for Maven downloads
	mavenCache := client.CacheVolume("maven-cache")

	// get reference to source code directory
	source := client.Host().Directory(".", dagger.HostDirectoryOpts{
		Exclude: []string{"ci"},
	})

	// use maven:3.9 container
	// mount cache and source code volumes
	// set working directory
	app := client.Container().
		From("maven:3.9-eclipse-temurin-17").
		WithMountedCache("~/.m2", mavenCache).
		WithMountedDirectory("/app", source).
		WithWorkdir("/app")

	// define binding between
	// application and service containers
	// define JDBC URL for tests
	// test, build and package application as JAR
	build := app.WithExec([]string{"mvn", "clean", "install"})

	// use eclipse alpine container as base
	// copy JAR files from builder
	// set entrypoint and database profile
	deploy := client.Container().
		From("eclipse-temurin:17-alpine").
		WithDirectory("/app", build.Directory("./target")).
		WithEntrypoint([]string{"java", "-jar", "/app/boot3.jar"})

	// publish image to registry
	address, err := deploy.WithRegistryAuth("docker.io", username, password).
		Publish(ctx, fmt.Sprintf("%s/boot3-maven", username))
	if err != nil {
		panic(err)
	}

	// print image address
	fmt.Println("Image published at:", address)
}
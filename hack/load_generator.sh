#!/usr/bin/env bash

for i in `seq 1 1000`;
    do
        kubectl create secret generic s${i}${i}${i} --from-literal=username=produser --from-literal=password=Y4nys7f11
    done
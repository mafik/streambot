@echo off

SET PATH=C:\w64devkit\bin;%PATH%

g++.exe main.cpp -L. -ltobii_gameintegration_x64 -o gaze.exe
gaze.exe
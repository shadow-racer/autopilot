#!/usr/bin/env python3
# -*- coding: utf-8 -*-

import busio
import time
from board import SCL, SDA

# Import the PCA9685 module.
from adafruit_pca9685 import PCA9685

CHANNEL = 4

# Create the I2C bus interface.
i2c_bus = busio.I2C(SCL, SDA)
 
# Create a simple PCA9685 class instance.
pca = PCA9685(i2c_bus)
 
# Set the PWM frequency to 60hz.
pca.frequency = 50
 
# Set the PWM duty cycle for channel CHANNEL to 50%. duty_cycle is 16 bits to match other PWM objects
# but the PCA9685 will only actually give 12 bits of resolution.
pca.channels[CHANNEL].duty_cycle = 0x7FFF
time.sleep(3)

pca.channels[CHANNEL].duty_cycle = 0xFFFF
time.sleep(3)

pca.channels[CHANNEL].duty_cycle = 0
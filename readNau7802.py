import PyNAU7802
import smbus2
import sched
import time
import mmap
import struct

import mmap
import struct
import os

# Define the shared memory format
FORMAT = 'f'  # Float format
SIZE = struct.calcsize(FORMAT)  # Size of the shared memory region

# Create a shared memory file
shm_file = os.open("my_shared_memory", os.O_CREAT | os.O_RDWR)

# Set the size of the shared memory file
os.ftruncate(shm_file, SIZE)

# Map the shared memory file into memory
shm = mmap.mmap(shm_file, SIZE)

# Create the bus
bus = smbus2.SMBus(0)

# Create the scale and initialize it
scale = PyNAU7802.NAU7802()
if scale.begin(bus):
    print("Connected!\n")
else:
    print("Can't find the scale, exiting ...\n")
    exit()

# Calculate the zero offset
print("Calculating the zero offset...")
scale.setGain(8)
scale.calculateZeroOffset()
print("The zero offset is: {0}\n".format(scale.getZeroOffset()))

print("Put a known mass on the scale.")
cal = 61274.138#float(input("Mass in kg? "))
scale.setCalibrationFactor(cal)
# Calculate the calibration factor
#scale.calculateCalibrationFactor(cal)


print("The calibration factor is: {0:0.3f}\n".format(scale.getCalibrationFactor()))

#input("Press [Enter] to start measuring masses. ")

# Initialize the scheduler
scheduler = sched.scheduler(time.time, time.sleep)
interval = 0.5


def measure_weight():
    weight = scale.getWeight()
        # Write data to the shared memory
    # Write data to the shared memory
    value = weight
    packed_data = struct.pack(FORMAT, value)

    # Write to the shared memory
    shm.seek(0)
    shm.write(packed_data)

    print("Raw: {0:0.3f} kg".format(weight))
    scheduler.enter(interval, 1, measure_weight)


# Start the measurements
scheduler.enter(0, 1, measure_weight)
scheduler.run()

# upp-opscop-utils
An area to store utilities useful for support

The files in this repo are meant to be run Individually, it is a storage are for scripts, splunk queries and any other information that is useful technically for support.


##  http-get-validator  ##

A simple script to call one of the upp store endpoints and check if content is present (and additional conditions). The script can serve as a basis to edit as circumstances require.

### To build:  ###
`go build`

### To run:  ###

minimal cmd:

`./http-getvalidator  --idListFile="your_input_file"`

Most of the arguments have default settings in the script.

Required:

- The end point you  want to run against.

- Basic auth header value. 

- Input text file (described below)

### checked in version functionality: ###

Parses a file of the following format:

`2017-05-17T16:36:01.625+0000,ef563223-61cd-42b2-ad1d-dec54d475f6d`

`2017-05-17T16:29:20.646+0000,172f4340-be0b-4f7f-a8de-4da3d944a898`

Makes a call to native store, extacts the lastModified filed and compares it to the date in the file

output files:
fail.txt- uuid not found in store
retry.txt- uuid found  date before file date
success.txt- uuid found and date after file input date.

=====================================================================



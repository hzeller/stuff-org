#!/usr/bin/env bash
##

if [ $# -ne 1 ]; then
   echo "Usage: $0 <outdir>"
   echo
   echo "Connects to locally attached camera and takes pictures,"
   echo "storing the result in a directory"
   exit 1
fi

FEH_GEOMETRY=2000x1338+560+0
FEH_GEOMETRY_SMALL=1000x670+0+0

RESULT_DIR=$1
if [ ! -d ${RESULT_DIR} ]; then
    echo "${RESULT_DIR} is not a directory. Please create that first"
    exit 1
fi
ID=""

TMP_PART=/tmp/part-no.$$
FEH_PICTURE=/tmp/feh-pic-$$.jpg

# Our constant process showing images. Makeing it auto-reload.
ln -sf $(realpath $(dirname $0)/../img/placeholder.jpg) $FEH_PICTURE
feh -g $FEH_GEOMETRY -R 0.2 "$FEH_PICTURE" &

mkdir -p $RESULT_DIR

while : ; do
    # Getting a valid part id
    while : ; do
	dialog --inputbox "Part ID" 10 20 "$ID" 2> $TMP_PART
	if [ $? -ne 0 ] ; then
	    echo "Exit"
	    exit
	fi
	ID=$(< $TMP_PART)
	if [ ! -z "$ID" ] ; then
	    break
	fi
    done

    # Display if available
    PIC_NAME="${RESULT_DIR}/${ID}.jpg"
    if [ -e "$PIC_NAME" ] ; then
	ln -sf $(realpath "$PIC_NAME") $FEH_PICTURE
	dialog --yesno "$PIC_NAME already exists.\nOverwrite ?" 10 30
	if [ $? -ne 0 ] ; then
	    continue
	fi
    fi

    # apparently we have to make sure that everything is deleted
    # on the camera before, otherwise we sometimes get the previous
    # image.
    gphoto2 -D && gphoto2 --capture-image-and-download
    mv capt0000.jpg "$PIC_NAME"
    ln -sf $(realpath "$PIC_NAME") "$FEH_PICTURE"

    # Be helpful and increment
    ID=$[$ID + 1]
done

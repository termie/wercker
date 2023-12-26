echo Start of script2.sh
if [ -e $FILENAMETOTEST ]
then
    echo "$FILENAMETOTEST exists"
else
    echo "$FILENAMETOTEST does not exist"
    # Cause docker run to return an error and fail the step
    exit 1
fi
echo End of script2.sh
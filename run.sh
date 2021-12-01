# run website
rm -rf website/content/post/
cp -r images website/static/ && cp -r article website/content/post/ && cd website && hugo server
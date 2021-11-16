# firebase login
# firebase init
# hugo && firebase deploy

# other way 
# github action

# push
# go run main.go
# echo "README索引生成完毕."

git status

time=$(date "+%Y-%m-%d %H:%M:%S")
echo "updateAt: ${time}"
while true;
do
    read -r -p "是否继续提交? [Y/n] " input

    case $input in
        [yY][eE][sS]|[yY])
            echo "继续提交"
            git add -A
            git commit -m "updateAt: ${time}"
            git push origin main
            exit 1
            ;;

        [nN][oO]|[nN])
            echo "中断提交"
            exit 1
            ;;

        *)
            echo "输入错误，请重新输入"
            ;;
    esac
done
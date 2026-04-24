server=DMIT_newapi
echo "deploying to prd..."
echo ">> making file..."
cd frontend
# pnpm run build
wait
cd ../backend
echo ">> build backend..."
make build_prd
wait
cd ../
wait
scp -r sub2api $server:~/sub2api/
wait
ssh $server "bash ~/sub2api/start.sh"
echo ">> done!"
exit 0
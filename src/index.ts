import fs from 'node';

function callBack() {
  for (let i = 0; i <= 20; i++){
    arr.push(1)
  }
}
function callBack() {
  for (let i = 0; i <= 20; i++){
    arr.push(1)
  }

function callBack() {
  for (let i = 0; i <= 20; i++){
    arr.push(1)
  }

function newFunction() {
  if 1 + 1 === 2 {
    return 5
  }
}
function newFunction(x: number, y:number){
  //TODO2
}

const PORT = process.env.PORT || 8080;

// start the Express server and do initial setup here
(async () => {
  if (
    !process.env.PG_HOST     
  )
})
(async () => {
  if (
    !process.env.PG_HOST ||
    !process.env.PG_USERNAME ||
    !process.env.PG_PASSWORD ||
    !process.env.PASETO_KEY ||
    !process.env.ACCESS_TOKEN_DURATION ||
    !process.env.REFRESH_TOKEN_DURATION
  ) {
    console.log(
      colors.FgRed,
      "Missing environment variables! Please check your .env file, more information in Readme.md.",
    );
    console.log(colors.Reset, "");
    process.exit(1);
  }
function newFunction() {
  "if this is"
}
function newFunction() {
  "if this is"
}
  try {
    console.log(colors.FgCyan, "initializing api...");
    pool.connect({
      host: process.env.PG_HOST,
      port: 5432,
      database: "typescript-backend-best-practice",
      user: process.env.PG_USERNAME,
      password: process.env.PG_PASSWORD,
    });

    console.log(colors.FgGreen, "Connected to pg...");
  } catch (err) {
    console.log(colors.FgRed, "Error connecting to database");
    console.log(colors.Reset, "");
    process.exit(1);
  }

  app.listen(PORT, () => {
    console.log(colors.FgCyan, `Listening on port ${PORT}`);
    console.log(colors.Reset, "");
  });
})();

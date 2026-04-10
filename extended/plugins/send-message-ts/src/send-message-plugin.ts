import { Server } from "./plugin.js";

const server = new Server();
const port = 9765;

server.listen(port).on("listening", () => {
  console.log(`send-message step plugin listening on :${port}`);
});

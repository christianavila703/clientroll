import { defineConfig } from "@rspack/cli";
import { rspack } from "@rspack/core";
import RefreshPlugin from "@rspack/plugin-react-refresh";
import path from "path";

const isDev = process.env.NODE_ENV === "development";

export default defineConfig({
  entry: {
    main: "./src/main.tsx",
  },
  output: {
    path: path.resolve(__dirname, "../web/assets"),
    filename: "[name].js",
    publicPath: "/assets/",
    clean: true,
  },
  resolve: {
    extensions: [".ts", ".tsx", ".js", ".jsx"],
    alias: {
      "@": path.resolve(__dirname, "src"),
    },
  },
  module: {
    rules: [
      {
        test: /\.tsx?$/,
        use: {
          loader: "builtin:swc-loader",
          options: {
            jsc: {
              parser: {
                syntax: "typescript",
                tsx: true,
              },
              transform: {
                react: {
                  runtime: "automatic",
                  development: isDev,
                  refresh: isDev,
                },
              },
            },
          },
        },
        type: "javascript/auto",
      },
      {
        test: /\.css$/,
        use: [
          {
            loader: "postcss-loader",
            options: {
              postcssOptions: {
                plugins: ["tailwindcss", "autoprefixer"],
              },
            },
          },
        ],
        type: "css",
      },
      {
        test: /\.(png|jpe?g|gif|svg|webp)$/i,
        type: "asset/resource",
      },
      {
        test: /\.(woff|woff2|eot|ttf|otf)$/i,
        type: "asset/resource",
      },
    ],
  },
  plugins: [
    isDev && new RefreshPlugin(),
    new rspack.HtmlRspackPlugin({
      template: "../web/index.html",
      inject: true,
    }),
  ].filter(Boolean),
  experiments: {
    css: true,
  },
  devServer: {
    port: 3000,
    hot: true,
    historyApiFallback: true,
    proxy: [
      {
        context: ["/api"],
        target: "http://localhost:8080",
        changeOrigin: true,
      },
    ],
  },
});

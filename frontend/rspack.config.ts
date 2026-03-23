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
    path: path.resolve(__dirname, "../web"),
    filename: "assets/[name].[contenthash:8].js",
    cssFilename: "assets/[name].[contenthash:8].css",
    publicPath: "/",
    clean: {
      keep: /^index\.html$/,
    },
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
        generator: {
          filename: "assets/images/[name].[contenthash:8][ext]",
        },
      },
      {
        test: /\.(woff|woff2|eot|ttf|otf)$/i,
        type: "asset/resource",
        generator: {
          filename: "assets/fonts/[name].[contenthash:8][ext]",
        },
      },
    ],
  },
  plugins: [
    isDev && new RefreshPlugin(),
    new rspack.DefinePlugin({
      __APP_BUILD_SHA__: JSON.stringify(process.env.APP_BUILD_SHA || ""),
      __APP_BUILD_TIME__: JSON.stringify(process.env.APP_BUILD_TIME || ""),
    }),
    new rspack.HtmlRspackPlugin({
      template: "./src/index.html",
      filename: "index.html",
      inject: true,
      minify: !isDev,
    }),
  ].filter(Boolean),
  devtool: isDev ? "eval-cheap-module-source-map" : false,
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

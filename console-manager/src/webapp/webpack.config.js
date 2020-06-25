const path = require('path');
const HtmlWebpackPlugin = require('html-webpack-plugin');
const dist = process.env.TARGET || path.resolve(__dirname, 'static');


module.exports = {
    entry: __dirname + '/src/main.js',
    output: {
        path: dist,
        filename: '[name].bundle.js',
        chunkFilename: '[name].bundle.js'
    },
    module: {
        rules: [{
            test: /\.css$/,
            use: ['style-loader', 'css-loader']
        }, {
            test: /\.js$/,
            use: ["source-map-loader"],
            enforce: "pre"
        }]
    },
    plugins: [
        new HtmlWebpackPlugin({
            title: 'Demo',
            filename: path.resolve(dist, 'index.html')
        })
    ]
};

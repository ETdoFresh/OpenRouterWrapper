require('dotenv').config();
const express = require('express');
const axios = require('axios');
const cors = require('cors');

const app = express();
const port = 5050;

// Middleware
app.use(cors());
app.use(express.json({limit: '100mb'}));
app.use(express.urlencoded({limit: '100mb', extended: true}));

// API endpoints
const OPENROUTER_API_URL = 'https://openrouter.ai/api/v1';
const DEEPSEEK_API_URL = 'https://api.deepseek.com/chat/completions';

// Chat completion endpoint
app.post('/v1/chat/completions', async (req, res) => {
    try {
        // Set response headers for streaming
        if (req.body.stream === true) {
            res.setHeader('Content-Type', 'text/event-stream');
            res.setHeader('Cache-Control', 'no-cache');
            res.setHeader('Connection', 'keep-alive');
        }

        // Check if model is deepseek/deepseek-chat or deepseek-chat
        if (req.body.model === 'deepseek/deepseek-chat') {
            try {
                // Update model to "deepseek-chat" for DeepSeek API
                const deepseekBody = {...req.body, model: 'deepseek-chat'};
                const deepseekResponse = await axios.post(DEEPSEEK_API_URL, deepseekBody, {
                    headers: {
                        'Authorization': `Bearer ${process.env.DEEPSEEK_API_KEY}`,
                        'Content-Type': 'application/json'
                    },
                    responseType: req.body.stream === true ? 'stream' : 'json'
                });

                if (req.body.stream === true) {
                    deepseekResponse.data.pipe(res);
                    return;
                }
                return res.json(deepseekResponse.data);
            } catch (error) {
                console.log('🏴‍☠️ DeepSeek API failed, falling back to OpenRouter');
            }
        }

        const response = await axios.post(`${OPENROUTER_API_URL}/chat/completions`, req.body, {
            headers: {
                'Authorization': `${req.headers['authorization']}`,
                'HTTP-Referer': req.headers['referer'] || 'http://localhost:5050',
                'X-Title': 'OpenRouter API Wrapper',
                'Content-Type': 'application/json'
            },
            responseType: req.body.stream === true ? 'stream' : 'json'
        });

        if (req.body.stream === true) {
            let retryCount = 0;
            const maxRetries = 3;
            const retryDelays = [500, 1000, 3000]; // Retry delays in ms
            let streamTimeout = null;
            let lastDataTime = Date.now();

            // Calculate exponential backoff delay
            const getRetryDelay = (attempt) => {
                return retryDelays[attempt] || retryDelays[retryDelays.length - 1];
            };

            const attemptStream = async () => {
                const resetStreamTimeout = (streamResponse) => {
                    if (streamTimeout) clearTimeout(streamTimeout);
                    lastDataTime = Date.now();
                    
                    streamTimeout = setTimeout(() => {
                        const timeSinceLastData = Date.now() - lastDataTime;
                        if (timeSinceLastData >= 15000 && retryCount < maxRetries) { // 15 seconds timeout
                            console.log(`🏴‍☠️ Stream timeout after ${timeSinceLastData}ms! Retry attempt ${retryCount + 1}/${maxRetries}`);
                            retryCount++;
                            streamResponse.data.destroy();
                            setTimeout(attemptStream, getRetryDelay(retryCount));
                        }
                    }, 15000); // Check every 15 seconds
                };

                try {
                    const streamResponse = await axios.post(`${OPENROUTER_API_URL}/chat/completions`, req.body, {
                        headers: {
                            'Authorization': `${req.headers['authorization']}`,
                            'HTTP-Referer': req.headers['referer'] || 'http://localhost:5050',
                            'X-Title': 'OpenRouter API Wrapper',
                            'Content-Type': 'application/json'
                        },
                        responseType: 'stream'
                    });

                    // Initial timeout for connection establishment
                    const initialTimeoutId = setTimeout(() => {
                        if (retryCount < maxRetries) {
                            console.log(`🏴‍☠️ Initial connection timeout! Retry attempt ${retryCount + 1}/${maxRetries}`);
                            retryCount++;
                            streamResponse.data.destroy();
                            setTimeout(attemptStream, getRetryDelay(retryCount));
                        }
                    }, 5000); // 5 seconds for initial connection

                    streamResponse.data.on('data', (chunk) => {
                        clearTimeout(initialTimeoutId);
                        resetStreamTimeout(streamResponse);
                        res.write(chunk);
                    });

                    streamResponse.data.on('end', () => {
                        if (streamTimeout) clearTimeout(streamTimeout);
                        clearTimeout(initialTimeoutId);
                        res.end();
                    });

                    streamResponse.data.on('error', (error) => {
                        if (streamTimeout) clearTimeout(streamTimeout);
                        clearTimeout(initialTimeoutId);
                        console.error('Stream error:', error);
                        if (retryCount < maxRetries) {
                            console.log(`🏴‍☠️ Stream error! Retry attempt ${retryCount + 1}/${maxRetries}`);
                            retryCount++;
                            setTimeout(attemptStream, getRetryDelay(retryCount));
                        } else {
                            console.error('All retry attempts failed!');
                            res.end();
                        }
                    });
                } catch (error) {
                    if (retryCount < maxRetries) {
                        console.log(`🏴‍☠️ Connection error! Retry attempt ${retryCount + 1}/${maxRetries}`);
                        retryCount++;
                        setTimeout(attemptStream, getRetryDelay(retryCount));
                    } else {
                        console.error('All retry attempts failed!');
                        res.status(500).json({ error: 'Failed to establish stream after maximum retries' });
                    }
                }
            };

            await attemptStream();
        } else {
            res.json(response.data);
        }
    } catch (error) {
        console.error('Error:', error.response?.status, error.response?.data?.statusMessage);
        const errorResponse = {
            error: {
                message: error.response?.data?.error || error.response?.data?.statusMessage || error.message || 'Internal server error',
                status: error.response?.status || 500,
                type: error.name || 'Error'
            }
        };
        res.status(errorResponse.error.status).json(errorResponse);
    }
});

// Generation stats endpoint
app.get('/v1/generation', async (req, res) => {
    try {
        const response = await axios.get(`${OPENROUTER_API_URL}/generation?id=${req.query.id}`, {
            headers: {
                'Authorization': `${req.headers['authorization']}`,
                'HTTP-Referer': req.headers['referer'] || 'http://localhost:5050',
                'X-Title': 'OpenRouter API Wrapper'
            }
        });
        res.json(response.data);
    } catch (error) {
        console.error('Error:', error.response?.status, error.response?.data?.statusMessage);
        const errorResponse = {
            error: {
                message: error.response?.data?.error || error.response?.data?.statusMessage || error.message || 'Internal server error',
                status: error.response?.status || 500,
                type: error.name || 'Error'
            }
        };
        res.status(errorResponse.error.status).json(errorResponse);
    }
});

// Models endpoint
app.get('/v1/models', async (req, res) => {
    try {
        const response = await axios.get(`${OPENROUTER_API_URL}/models`, {
            headers: {
                'Authorization': `${req.headers['authorization']}`,
                'HTTP-Referer': req.headers['referer'] || 'http://localhost:5050',
                'X-Title': 'OpenRouter API Wrapper'
            }
        });
        res.json(response.data);
    } catch (error) {
        console.error('Error:', error.response?.status, error.response?.data?.statusMessage);
        const errorResponse = {
            error: {
                message: error.response?.data?.error || error.response?.data?.statusMessage || error.message || 'Internal server error',
                status: error.response?.status || 500,
                type: error.name || 'Error'
            }
        };
        res.status(errorResponse.error.status).json(errorResponse);
    }
});

app.listen(port, () => {
    console.log(`🏴‍☠️ Server be sailin' on port ${port}! Arrr!`);
});

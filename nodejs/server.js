require('dotenv').config();
const express = require('express');
const axios = require('axios');
const cors = require('cors');
const fs = require('fs');
const path = require('path');

const app = express();
const port = 5050;

// Helper function to save history
const saveHistory = (data, prefix) => {
    // Create history directory if it doesn't exist
    if (!fs.existsSync('history')) {
        fs.mkdirSync('history', { recursive: true });
    }

    // Generate filename with timestamp and prefix
    const timestamp = new Date().toISOString().replace(/[-:.]/g, '');
    const filename = `${timestamp}-${prefix}.json`;
    const filePath = path.join('history', filename);
    const tempFilePath = path.join('history', `temp-${Date.now()}.json`);

    try {
        // Parse and re-encode data to remove escaping
        const processedData = JSON.parse(JSON.stringify(data));
        
        // Write to temp file
        fs.writeFileSync(tempFilePath, JSON.stringify(processedData, null, 2), 'utf8');
        
        // Rename temp file to final destination
        fs.renameSync(tempFilePath, filePath);
    } catch (error) {
        // Clean up temp file if rename failed
        if (fs.existsSync(tempFilePath)) {
            try {
                fs.unlinkSync(tempFilePath);
            } catch (cleanupError) {
                console.error('Error cleaning up temp file:', cleanupError);
            }
        }
        throw error;
    }
};

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
        // Save request history
        saveHistory({
            method: req.method,
            url: req.originalUrl,
            headers: req.headers,
            body: req.body
        }, 'request');
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
                    responseType: 'json'
                });

                // Save response history
                saveHistory({
                    status: deepseekResponse.status,
                    headers: deepseekResponse.headers,
                    data: deepseekResponse.data
                }, 'response');

                return res.json(deepseekResponse.data);
            } catch (error) {
                console.log('🏴‍☠️ DeepSeek API failed, falling back to OpenRouter');
            }
        }

        let lastError = null;
        for (let attempt = 0; attempt < 3; attempt++) {
            try {
                const response = await axios.post(`${OPENROUTER_API_URL}/chat/completions`, req.body, {
                    headers: {
                        'Authorization': `${req.headers['authorization']}`,
                        'HTTP-Referer': req.headers['referer'] || 'http://localhost:5050',
                        'X-Title': 'OpenRouter API Wrapper',
                        'Content-Type': 'application/json'
                    },
                    responseType: 'stream'
                });

                let finalResponse = {
                    id: '',
                    object: 'chat.completion',
                    created: Math.floor(Date.now() / 1000),
                    model: req.body.model,
                    choices: [],
                    usage: {
                        prompt_tokens: 0,
                        completion_tokens: 0,
                        total_tokens: 0
                    }
                };

                // Process streamed response
                response.data.on('data', chunk => {
                    const data = chunk.toString();
                    try {
                        const jsonData = JSON.parse(data);
                        
                        // Initialize final response on first chunk
                        if (!finalResponse.id) {
                            finalResponse.id = jsonData.id;
                            finalResponse.choices = jsonData.choices.map(choice => ({
                                index: choice.index,
                                message: {
                                    role: '',
                                    content: ''
                                },
                                finish_reason: ''
                            }));
                        }

                        // Accumulate content for each choice
                        jsonData.choices.forEach((choice, i) => {
                            if (choice.delta?.content) {
                                finalResponse.choices[i].message.content += choice.delta.content;
                            }
                            if (choice.delta?.role) {
                                finalResponse.choices[i].message.role = choice.delta.role;
                            }
                            if (choice.finish_reason) {
                                finalResponse.choices[i].finish_reason = choice.finish_reason;
                            }
                        });
                    } catch (error) {
                        console.error('Error parsing chunk:', error);
                    }
                });

                response.data.on('end', () => {
                    // Save complete response history
                    saveHistory({
                        status: response.status,
                        headers: response.headers,
                        data: finalResponse
                    }, 'response');

                    res.json(finalResponse);
                });

                response.data.on('error', error => {
                    console.error('Stream error:', error);
                    res.status(500).json({
                        error: {
                            message: 'Stream processing error',
                            status: 500,
                            type: 'StreamError'
                        }
                    });
                });
            } catch (error) {
                lastError = error;
                console.log(`🏴‍☠️ Connection error! Retry attempt ${attempt + 1}/3`);
                await new Promise(resolve => setTimeout(resolve, [500, 1000, 3000][attempt]));
            }
        }

        console.error('All retry attempts failed!');
        const errorResponse = {
            error: {
                message: lastError.response?.data?.error || lastError.message || 'Internal server error',
                status: lastError.response?.status || 500,
                type: lastError.name || 'Error'
            }
        };
        res.status(errorResponse.error.status).json(errorResponse);
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

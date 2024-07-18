#pragma once

#include <inttypes.h>

namespace TobiiGameIntegration
{
    struct ITobiiGameIntegrationApi;

    class GameIntegrationLoader
    {
    public:
        inline GameIntegrationLoader();
        inline ~GameIntegrationLoader();
        inline bool	IsLoaded() const;
        inline bool	IsLinked() const;

        // If returns false - check GetLastErrorFromWindows()
        bool Load(const char* dllPath);
        bool Link();
        void Unload();
        DWORD GetLastErrorFromWindows();
        ITobiiGameIntegrationApi* LinkAndGetApi(const char* titleName, const char* dllPath, int majorVersion, int minorVersion, int revision, const uint16_t* license, uint32_t licenseSize, bool analyticalUse);
    protected:
        bool m_Linked;
        void* m_hInstance;
        typedef ITobiiGameIntegrationApi* (__cdecl *GetApiFunction)(const char* titleName, int majorVersion, int minorVersion, int revision, const uint16_t* license, uint32_t licenseSize, bool analyticalUse);
        GetApiFunction m_GetApiFunctionPointer;
        DWORD m_LastError;
    };

    GameIntegrationLoader::GameIntegrationLoader() : m_Linked(false), m_hInstance(nullptr), m_LastError(0) {};
    GameIntegrationLoader::~GameIntegrationLoader() { Unload(); };
    bool GameIntegrationLoader::IsLoaded() const { return(m_hInstance != nullptr); }
    bool GameIntegrationLoader::IsLinked() const { return(m_Linked); }
}

